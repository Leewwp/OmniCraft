package llm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// SP-25 低-12：SSE 流单行 data: 可远超 bufio.Scanner 默认 64KB（一次性长
// tool_call arguments / 超长单块内容）。上限升至 4MB 前，超长行触发
// ErrTooLong → 整流失败 → 路由回退。三家流式 provider 同一回归。
func oversizeLineStreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	big := strings.Repeat("长", 40*1024) // ~120KB UTF-8 单行 content
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n", big)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n")
		io.WriteString(w, "data: [DONE]\n")
	}))
}

func TestStreamHandlesOversizeSingleLine(t *testing.T) {
	t.Run("openai_compat", func(t *testing.T) {
		srv := oversizeLineStreamServer(t)
		defer srv.Close()
		p := NewOpenAICompatProvider("test-key", srv.URL, "deepseek-chat", "embed")
		var runes int
		err := p.ChatStream(context.Background(), ChatRequest{
			Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		}, func(delta ChatDelta) error {
			runes += len(delta.Content)
			return nil
		})
		if err != nil {
			t.Fatalf("oversize single line must not kill the stream: %v", err)
		}
		if runes == 0 {
			t.Fatal("expected the oversize delta to be delivered")
		}
	})

	t.Run("minimax", func(t *testing.T) {
		srv := oversizeLineStreamServer(t)
		defer srv.Close()
		p := NewMiniMaxProvider("test-key", srv.URL, "MiniMax-M1", "embo-01")
		var runes int
		err := p.ChatStream(context.Background(), ChatRequest{
			Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		}, func(delta ChatDelta) error {
			runes += len(delta.Content)
			return nil
		})
		if err != nil {
			t.Fatalf("oversize single line must not kill the stream: %v", err)
		}
		if runes == 0 {
			t.Fatal("expected the oversize delta to be delivered")
		}
	})

	t.Run("qwen", func(t *testing.T) {
		// qwen 走 DashScope 原生响应形态（output.choices[].message）。
		big := strings.Repeat("长", 40*1024)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"output\":{\"choices\":[{\"message\":{\"content\":\"%s\"}}]}}\n", big)
			io.WriteString(w, "data: {\"output\":{\"choices\":[{\"finish_reason\":\"stop\"}]},\"usage\":{\"total_tokens\":42}}\n")
			io.WriteString(w, "data: [DONE]\n")
		}))
		defer srv.Close()
		t.Setenv("AGENT_LLM_API_BASE", srv.URL)
		p := NewQwenProvider("test-key", "qwen-plus", "text-embedding-v4")
		var runes int
		err := p.ChatStream(context.Background(), ChatRequest{
			Messages: []ChatMessage{{Role: "user", Content: "hi"}},
		}, func(delta ChatDelta) error {
			runes += len(delta.Content)
			return nil
		})
		if err != nil {
			t.Fatalf("oversize single line must not kill the stream: %v", err)
		}
		if runes == 0 {
			t.Fatal("expected the oversize delta to be delivered")
		}
	})
}
