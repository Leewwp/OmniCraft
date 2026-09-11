package llm

import (
	"context"
	"fmt"
)

// BatchEmbeddingProvider is the optional batch embedding capability (DashScope
// v4 batch endpoint, 10 texts per request enforced by the provider). Chunk
// projection and multi-query embedding discover it through a type assertion,
// so any wrapper that hides it silently degrades full re-embedding to one
// HTTP call per text.
type BatchEmbeddingProvider interface {
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

// CompositeProvider splits the chat and embedding surfaces across two
// delegate providers (canonical profile: MiniMax chat through the minimax
// adapter — tool calls plus <think> splitting — with DashScope
// text-embedding-v4 through openai_compat). Every surface delegates to the
// adapter that natively owns it, so wire behavior and the per-delegate OTel
// spans (gen_ai.request.model) stay exactly as they are on direct wiring.
type CompositeProvider struct {
	chat  LLMProvider
	embed LLMProvider
}

func NewCompositeProvider(chat, embed LLMProvider) *CompositeProvider {
	return &CompositeProvider{chat: chat, embed: embed}
}

func (p *CompositeProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	return p.chat.Chat(ctx, req)
}

func (p *CompositeProvider) ChatStream(ctx context.Context, req ChatRequest, handler func(delta ChatDelta) error) error {
	return p.chat.ChatStream(ctx, req, handler)
}

func (p *CompositeProvider) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	return p.embed.GetEmbedding(ctx, text)
}

// GetEmbeddings forwards the batch endpoint so full re-embedding keeps the
// v4 batch contract instead of degrading to per-text requests.
func (p *CompositeProvider) GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if batch, ok := p.embed.(BatchEmbeddingProvider); ok {
		return batch.GetEmbeddings(ctx, texts)
	}
	return nil, fmt.Errorf("embedding provider %T has no batch embedding endpoint", p.embed)
}

// Model reports the chat-side model identity (agent-smoke cost estimates and
// evidence reports).
func (p *CompositeProvider) Model() string {
	if identity, ok := p.chat.(interface{ Model() string }); ok {
		return identity.Model()
	}
	return ""
}

// EmbeddingModel reports the embedding-side model identity.
func (p *CompositeProvider) EmbeddingModel() string {
	if identity, ok := p.embed.(interface{ Model() string }); ok {
		return identity.Model()
	}
	return ""
}
