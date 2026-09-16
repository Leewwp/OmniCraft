package llm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

// AgentModelOption is the wire shape served by the agent models endpoint:
// a registered model id plus its display name.
type AgentModelOption struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// ModelRouter is the capability surface a RoutingProvider exposes to callers
// (model preference validation and the selectable option list). Services
// discover it by interface assertion so single-provider wirings stay unchanged.
type ModelRouter interface {
	ModelOptions() []AgentModelOption
	ModelRegistered(id string) bool
}

// retry reasons (cfg.Agent.Routing.RetryOn). Deterministic signals only —
// content-level heuristics are explicitly out of scope (#544 user ruling).
const (
	RetryOnProviderError = "provider_error"
	RetryOnBlankAnswer   = "blank_answer"
)

// RoutingProvider wraps a registry of providers behind one LLMProvider face
// (#545): requests run down an ordered chain (ChatRequest.ModelPref first
// when registered, then the configured primary + fallbacks). Failover rules:
//   - provider_error: retry the next model only while nothing observable has
//     streamed (no content, no tool calls) — once an answer started flowing,
//     surfacing the error keeps the existing degraded semantics and never
//     concatenates two half-answers.
//   - blank_answer: a stream that completed without error but produced no
//     tool calls and a whitespace-only body (#544 form A: reasoning burned,
//     body blank) retries on the next model. Think deltas already forwarded
//     stay concatenated client-side; they are display-only.
//
// context.Canceled never retries. When the chain is exhausted the last
// outcome is returned unchanged.
type RoutingProvider struct {
	order      []string // primary first, then fallbacks (registered only)
	models     map[string]LLMProvider
	options    []AgentModelOption
	retryErr   bool
	retryBlank bool
}

// NewRoutingProvider builds the routing surface. chain entries missing from
// models are dropped; embeddings always delegate to the first chain entry so
// the canonical split wiring (chat router + embedding adapter) keeps working.
func NewRoutingProvider(models map[string]LLMProvider, chain []string, retryOn []string, options []AgentModelOption) *RoutingProvider {
	order := make([]string, 0, len(chain))
	seen := make(map[string]bool, len(chain))
	for _, name := range chain {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		if _, ok := models[name]; !ok {
			continue
		}
		seen[name] = true
		order = append(order, name)
	}
	rp := &RoutingProvider{
		order:      order,
		models:     models,
		retryErr:   containsFold(retryOn, RetryOnProviderError),
		retryBlank: containsFold(retryOn, RetryOnBlankAnswer),
		options:    options,
	}
	if len(rp.order) == 0 {
		slog.Warn("agent model routing enabled but no registered model is in the chain")
	}
	return rp
}

func containsFold(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), want) {
			return true
		}
	}
	return false
}

// ModelOptions lists selectable models in chain order.
func (p *RoutingProvider) ModelOptions() []AgentModelOption { return p.options }

// ModelRegistered reports whether an id can be pinned via ChatRequest.ModelPref.
func (p *RoutingProvider) ModelRegistered(id string) bool {
	_, ok := p.models[strings.TrimSpace(id)]
	return ok
}

// resolveChain puts a registered preference first, keeping the rest of the
// configured chain behind it (polyu-agent "preferred model" semantics).
func (p *RoutingProvider) resolveChain(pref string) []string {
	pref = strings.TrimSpace(pref)
	if pref == "" {
		return p.order
	}
	if _, ok := p.models[pref]; !ok {
		return p.order
	}
	chain := make([]string, 0, len(p.order))
	chain = append(chain, pref)
	for _, name := range p.order {
		if name != pref {
			chain = append(chain, name)
		}
	}
	return chain
}

func (p *RoutingProvider) primary() LLMProvider {
	if len(p.order) == 0 {
		return nil
	}
	return p.models[p.order[0]]
}

func (p *RoutingProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	if primary := p.primary(); primary != nil {
		return primary.GetEmbedding(ctx, text)
	}
	return nil, errors.New("agent model routing has no registered model")
}

func (p *RoutingProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	chain := p.resolveChain(req.ModelPref)
	var resp *ChatResponse
	var err error
	for i, name := range chain {
		resp, err = p.models[name].Chat(ctx, req)
		if err == nil {
			if strings.TrimSpace(resp.Content) != "" || len(resp.ToolCalls) > 0 || !p.retryBlank || i == len(chain)-1 {
				return resp, nil
			}
			p.route(name, chain[i+1], RetryOnBlankAnswer)
			continue
		}
		if errors.Is(err, context.Canceled) || !p.retryErr || i == len(chain)-1 {
			return resp, err
		}
		p.route(name, chain[i+1], RetryOnProviderError)
	}
	return resp, err
}

func (p *RoutingProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	chain := p.resolveChain(req.ModelPref)
	var lastErr error
	for i, name := range chain {
		var content strings.Builder
		sawToolCalls := false
		err := p.models[name].ChatStream(ctx, req, func(delta ChatDelta) error {
			if delta.Content != "" {
				content.WriteString(delta.Content)
			}
			if len(delta.ToolCalls) > 0 {
				sawToolCalls = true
			}
			return handler(delta)
		})
		if err == nil {
			blank := !sawToolCalls && strings.TrimSpace(content.String()) == ""
			if !blank || !p.retryBlank || i == len(chain)-1 {
				return nil
			}
			p.route(name, chain[i+1], RetryOnBlankAnswer)
			continue
		}
		lastErr = err
		if errors.Is(err, context.Canceled) {
			return err
		}
		observableStreamed := sawToolCalls || strings.TrimSpace(content.String()) != ""
		if !p.retryErr || i == len(chain)-1 || observableStreamed {
			return err
		}
		p.route(name, chain[i+1], RetryOnProviderError)
	}
	return lastErr
}

func (p *RoutingProvider) route(from, to, reason string) {
	slog.Warn("agent model routed", "from", from, "to", to, "reason", reason)
}
