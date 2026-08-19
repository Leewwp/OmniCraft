package container

import (
	"context"

	"omnicraft/backend/internal/service"
	ragservice "omnicraft/backend/internal/service/rag"
)

// agentRAGRetriever keeps the AgentService boundary independent from the RAG
// projection package, whose projection code depends on the base service
// package. The adapter is the only place where the two result contracts meet.
type agentRAGRetriever struct {
	retriever *ragservice.HybridRetriever
}

func (r *agentRAGRetriever) Retrieve(ctx context.Context, query string, viewerID int64) (service.AgentRetrievalResult, error) {
	result, err := r.retriever.Retrieve(ctx, query, viewerID)
	if err != nil {
		return service.AgentRetrievalResult{}, err
	}
	candidates := make([]service.AgentRetrievalCandidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		candidates = append(candidates, service.AgentRetrievalCandidate{
			ChunkKey:        candidate.ChunkKey,
			ContentID:       candidate.ContentID,
			ContentVersion:  candidate.ContentVersion,
			ChunkIndex:      candidate.ChunkIndex,
			ChunkingVersion: candidate.ChunkingVersion,
			IndexVersion:    candidate.IndexVersion,
			Title:           candidate.Title,
			Heading:         candidate.Heading,
			Text:            candidate.Text,
			Zone:            candidate.Zone,
			ContentType:     candidate.ContentType,
			Source:          candidate.Source,
		})
	}
	return service.AgentRetrievalResult{Candidates: candidates, Degraded: result.Degraded}, nil
}
