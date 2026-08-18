package rag

import "context"

type EmbeddingProvider interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
}

type ProviderChunkEmbedder struct {
	provider EmbeddingProvider
}

func NewProviderChunkEmbedder(provider EmbeddingProvider) *ProviderChunkEmbedder {
	return &ProviderChunkEmbedder{provider: provider}
}

func (e *ProviderChunkEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
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
