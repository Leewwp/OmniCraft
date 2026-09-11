package rag

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type deterministicProvider struct{ calls []string }

func (p *deterministicProvider) GetEmbedding(_ context.Context, text string) ([]float32, error) {
	p.calls = append(p.calls, text)
	vector := make([]float32, 1536)
	vector[0] = float32(len([]rune(text)))
	return vector, nil
}

func TestProviderChunkEmbedderEmbedsEveryChunkInOrder(t *testing.T) {
	provider := &deterministicProvider{}
	embedder := NewProviderChunkEmbedder(provider)
	vectors, err := embedder.Embed(context.Background(), []string{"first", "second"})
	require.NoError(t, err)
	require.Equal(t, []string{"first", "second"}, provider.calls)
	require.Len(t, vectors, 2)
	require.Len(t, vectors[0], 1536)
	require.Equal(t, float32(5), vectors[0][0])
}
