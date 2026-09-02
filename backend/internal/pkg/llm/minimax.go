package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"omnicraft/backend/internal/observability"
)

// MiniMaxProvider talks to the MiniMax platform (api.minimaxi.com). Chat uses
// the OpenAI-compatible /v1/chat/completions wire format supported by the
// current M-series models. Embeddings use the
// MiniMax-specific wire format: /v1/embeddings requires a "texts" array plus
// a "type" field (the endpoint accepts "query" and rejects "document") and
// returns a "vectors" array instead of OpenAI's data[].embedding.
//
// MiniMax M-series models can emit <think>...</think> reasoning blocks inside content. They
// are stripped here so chain-of-thought never reaches the UI.
type MiniMaxProvider struct {
	openAI     *OpenAICompatProvider
	embedModel string
}

func NewMiniMaxProvider(apiKey, apiBase, model, embedModel string, opts ...ProviderOption) *MiniMaxProvider {
	if apiBase == "" {
		apiBase = "https://api.minimaxi.com"
	}
	openAI := NewOpenAICompatProvider(apiKey, apiBase, model, embedModel, opts...)
	openAI.system = "minimax"
	return &MiniMaxProvider{
		openAI:     openAI,
		embedModel: embedModel,
	}
}

// Model reports the configured chat model (used by the opt-in agent-smoke
// release-evidence command).
func (p *MiniMaxProvider) Model() string { return p.openAI.Model() }

func (p *MiniMaxProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	resp, err := p.openAI.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	resp.Content = stripThink(resp.Content)
	return resp, nil
}

func (p *MiniMaxProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) (err error) {
	ctx, span := startLLMSpan(ctx, "chat.stream", p.openAI.model, "minimax", req.Temperature)
	var lastUsage *TokenUsage
	defer func() { finishLLMSpan(span, err, lastUsage) }()
	payload := openAIRequest{
		Model:       p.openAI.model,
		Messages:    req.Messages,
		Tools:       req.Tools,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      true,
		// MiniMax omits usage on streamed completions unless explicitly
		// requested; the Agent usage event needs the token accounting.
		StreamOptions: &streamOptions{IncludeUsage: true},
	}
	resp, started, err := p.openAI.doPost(ctx, "/v1/chat/completions", payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("minimax api error %d", resp.StatusCode)
	}

	// MiniMax closes the stream after a chunk carrying a non-empty
	// finish_reason ("stop" for plain answers, "tool_calls" for tool rounds)
	// without sending a [DONE] sentinel, so the loop terminates on that chunk
	// instead of demanding an OpenAI-style terminator.
	splitter := newThinkSplitter()
	sawFinish := false
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			return handler(ChatDelta{Done: true})
		}
		var chunk openAIResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		// OpenAI-compatible streams deliver usage as a terminal chunk with an
		// empty choices array once include_usage is requested. Surface it
		// before the choices guard so token accounting is not dropped.
		if usage := usageFromRaw(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens); usage != nil {
			lastUsage = usage
			if err := handler(ChatDelta{Usage: usage}); err != nil {
				return err
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		choice := chunk.Choices[0]
		thinking, content := splitter.split(choice.Delta.Content)
		delta := ChatDelta{
			Content:   content,
			Thinking:  thinking,
			ToolCalls: choice.Delta.ToolCalls,
			Usage:     usageFromRaw(chunk.Usage.PromptTokens, chunk.Usage.CompletionTokens),
			Done:      choice.FinishReason == "stop",
		}
		if delta.Usage != nil {
			lastUsage = delta.Usage
		}
		if choice.FinishReason != "" {
			sawFinish = true
			flushThink, flushContent := splitter.splitFlush()
			delta.Thinking += flushThink
			delta.Content += flushContent
		}
		if err := handler(delta); err != nil {
			return err
		}
		// Do not return on the finish chunk: with include_usage requested the
		// terminal usage-only chunk arrives after it, and the stream ends at
		// EOF.
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawFinish {
		return fmt.Errorf("minimax stream ended before completion")
	}
	return nil
}

type minimaxEmbeddingRequest struct {
	Model string   `json:"model"`
	Texts []string `json:"texts"`
	Type  string   `json:"type"`
}

type minimaxEmbeddingResponse struct {
	Vectors  [][]float32 `json:"vectors"`
	BaseResp struct {
		StatusCode int    `json:"status_code"`
		StatusMsg  string `json:"status_msg"`
	} `json:"base_resp"`
}

// GetEmbedding uses the MiniMax text-embedding model (e.g. embo-01, 1536
// dims). The endpoint requires type "query"; "document" is rejected, so both
// indexing and query embedding use the same type to keep one vector space.
func (p *MiniMaxProvider) GetEmbedding(ctx context.Context, text string) (embedding []float32, err error) {
	ctx, span := startLLMSpan(ctx, "embedding", p.embedModel, "minimax", 0)
	defer func() { finishLLMSpan(span, err, nil) }()
	payload := minimaxEmbeddingRequest{Model: p.embedModel, Texts: []string{text}, Type: "query"}
	path := "/v1/embeddings"
	if p.openAI.embedGroupID != "" {
		path += "?GroupId=" + url.QueryEscape(p.openAI.embedGroupID)
	}
	resp, started, err := p.openAI.doPostAt(ctx, p.openAI.embedAPIBase, path, payload)
	defer func() { observability.ObserveExternalCall("llm", started, err) }()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("minimax embedding api error %d", resp.StatusCode)
	}
	var result minimaxEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.BaseResp.StatusCode != 0 {
		return nil, fmt.Errorf("minimax embedding error %d: %s", result.BaseResp.StatusCode, result.BaseResp.StatusMsg)
	}
	if len(result.Vectors) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}
	return result.Vectors[0], nil
}

const thinkOpen = "<think>"
const thinkClose = "</think>"

// thinkSplitter routes MiniMax reasoning blocks into a separate display-only
// channel instead of discarding them. Blocks may span stream chunks, so a
// stateful buffer keeps text inside an unclosed block until the closing tag
// arrives or the stream ends.
type thinkSplitter struct {
	buf   string
	depth int
}

func newThinkSplitter() *thinkSplitter { return &thinkSplitter{} }

// split consumes one content chunk and returns the (thinking, content)
// increments that are safe to emit immediately; a trailing fragment that
// could start a tag in a later chunk stays buffered.
func (s *thinkSplitter) split(in string) (thinking, content string) {
	if in == "" {
		return "", ""
	}
	if s.buf == "" && s.depth == 0 && !strings.Contains(in, "<") {
		return "", in
	}
	s.buf += in
	var think strings.Builder
	var body strings.Builder
	for len(s.buf) > 0 {
		openIdx := strings.Index(s.buf, thinkOpen)
		closeIdx := strings.Index(s.buf, thinkClose)
		if openIdx < 0 && closeIdx < 0 {
			// No complete tag: route everything except a trailing fragment
			// that could begin a tag in a later chunk.
			cut := trailingTagPrefix(s.buf)
			visible := s.buf[:len(s.buf)-cut]
			if s.depth == 0 {
				body.WriteString(visible)
			} else {
				think.WriteString(visible)
			}
			s.buf = s.buf[len(s.buf)-cut:]
			return think.String(), body.String()
		}

		nextIdx := openIdx
		nextTag := thinkOpen
		if nextIdx < 0 || (closeIdx >= 0 && closeIdx < nextIdx) {
			nextIdx = closeIdx
			nextTag = thinkClose
		}
		if s.depth == 0 {
			body.WriteString(s.buf[:nextIdx])
		} else {
			think.WriteString(s.buf[:nextIdx])
		}
		s.buf = s.buf[nextIdx+len(nextTag):]
		if nextTag == thinkOpen {
			s.depth++
		} else if s.depth > 0 {
			s.depth--
		}
	}
	return think.String(), body.String()
}

// splitFlush returns what is left at the end of a stream: body text before an
// unclosed think block, and the unclosed reasoning remainder as thinking
// (display-only, so an unterminated block is not lost).
func (s *thinkSplitter) splitFlush() (thinking, content string) {
	if len(s.buf) == 0 {
		return "", ""
	}
	if s.depth == 0 {
		out := s.buf
		if cut := trailingTagPrefix(s.buf); cut > 0 {
			out = s.buf[:len(s.buf)-cut]
		}
		s.buf = ""
		return "", out
	}
	think := s.buf
	s.buf = ""
	s.depth = 0
	return think, ""
}

// trailingTagPrefix returns the length of the longest suffix of s that is a
// prefix of "<think>" or "</think>" (a tag fragment split across chunks).
func trailingTagPrefix(s string) int {
	max := len(s)
	if max > len(thinkClose) {
		max = len(thinkClose)
	}
	for k := max; k >= 1; k-- {
		suffix := s[len(s)-k:]
		if strings.HasPrefix(thinkOpen, suffix) || strings.HasPrefix(thinkClose, suffix) {
			return k
		}
	}
	return 0
}

func stripThink(content string) string {
	s := newThinkSplitter()
	_, body := s.split(content)
	_, flushBody := s.splitFlush()
	return body + flushBody
}
