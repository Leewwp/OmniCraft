package rag

import "context"

type EmbeddingProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

// batchEmbeddingProvider is the optional batch capability (DashScope v4
// batch endpoint, 10 texts per request enforced inside the provider).
type batchEmbeddingProvider interface {
	GetEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
}

type ProviderChunkEmbedder struct {
	provider EmbeddingProvider
}

func NewProviderChunkEmbedder(provider EmbeddingProvider) *ProviderChunkEmbedder {
	return &ProviderChunkEmbedder{provider: provider}
}

// Embed embeds chunk texts, preferring the provider batch endpoint when
// available (A-03: full re-embedding goes through batched v4 calls) and
// falling back to one call per text for legacy providers.
func (e *ProviderChunkEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if batch, ok := e.provider.(batchEmbeddingProvider); ok {
		vectors, err := batch.GetEmbeddings(ctx, texts)
		if err == nil && len(vectors) == len(texts) {
			return vectors, nil
		}
		if err != nil {
			return nil, err
		}
	}
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vector, err := e.provider.GetEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vector
	}
	return vectors, nil
}
