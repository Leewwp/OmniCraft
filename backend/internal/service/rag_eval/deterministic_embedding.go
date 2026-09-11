package rageval

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// DeterministicEmbedding produces a stable dim-dimensional vector from text
// via sha256 token hashing. It is the local-dev standin for the real
// embedding provider (rag-minimal-slice T01; no provider credentials in local
// development mode): the pgvector retrieval path is exercised end-to-end and
// deterministically, but the vectors are lexical, not semantic. Artifact
// environment metadata always records query_embedding_provider so the standin
// is never mistaken for a real semantic embedding.
func DeterministicEmbedding(text string, dims int) []float32 {
	tokens := wordTokens(text)
	if len(tokens) == 0 || dims <= 0 {
		return make([]float32, dims)
	}
	vector := make([]float64, dims)
	weight := 1.0 / float64(len(tokens))
	for _, token := range tokens {
		sum := sha256.Sum256([]byte(token))
		idx := int(binary.LittleEndian.Uint32(sum[0:4])) % dims
		sign := 1.0
		if sum[4]&1 == 1 {
			sign = -1.0
		}
		vector[idx] += sign * weight
	}
	var norm float64
	for _, v := range vector {
		norm += v * v
	}
	if norm > 0 {
		scale := 1.0 / math.Sqrt(norm)
		for i := range vector {
			vector[i] *= scale
		}
	}
	result := make([]float32, dims)
	for i, v := range vector {
		result[i] = float32(v)
	}
	return result
}
