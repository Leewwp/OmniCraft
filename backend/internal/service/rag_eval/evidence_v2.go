package rageval

import (
	"sort"

	"omnicraft/backend/internal/model"
)

// SpanResolution is the eval-time resolution of one annotated evidence span
// against the current chunking (contract §3/H5): the code point half-open
// interval [start,end) is re-resolved onto the chunks that exist under the
// current chunking_version. The frozen chunk_key snapshot is cross-checked
// for drift reporting only — it is never treated as truth.
type SpanResolution struct {
	ContentID     int64    `json:"content_id"`
	SpanStart     int      `json:"span_start"`
	SpanEnd       int      `json:"span_end"`
	ResolvedKeys  []string `json:"resolved_chunk_keys,omitempty"`
	SnapshotKey   string   `json:"snapshot_chunk_key,omitempty"`
	SnapshotDrift bool     `json:"snapshot_drift"`
	// Degenerate marks a (0,0)-style placeholder span: v1 rows carry no real
	// offsets, and v2 freezing rejects them (freeze gate: no 0..400
	// placeholders), so resolution is skipped instead of producing noise.
	Degenerate bool `json:"degenerate"`
	// Resolved reports whether at least one current chunk overlaps the span.
	Resolved bool `json:"resolved"`
}

// ResolveSpanToChunks returns the current chunks whose own source interval
// [chunk.SourceStart, chunk.SourceEnd) overlaps the annotated span
// [span.SourceStart, span.SourceEnd) on the same content. Overlap uses the
// standard half-open intersection test (startA < endB && startB < endA); a
// zero-length span or zero-length chunk therefore never matches.
func ResolveSpanToChunks(span EvidenceSpan, chunks []model.RagChunk) []model.RagChunk {
	if span.SourceStart < 0 || span.SourceEnd <= span.SourceStart {
		return nil
	}
	var matched []model.RagChunk
	for _, chunk := range chunks {
		if chunk.ContentID != span.ContentID {
			continue
		}
		if span.SourceStart < chunk.SourceEnd && chunk.SourceStart < span.SourceEnd {
			matched = append(matched, chunk)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ChunkIndex < matched[j].ChunkIndex })
	return matched
}

// ChunkSource loads the current chunks of one content item under the
// chunking_version being evaluated. The eval harness wires this to the rag
// chunk repository; unit tests supply in-memory slices.
type ChunkSource func(contentID int64) ([]model.RagChunk, error)

// ResolveEvidenceSpans resolves every span against the ChunkSource and
// cross-checks the frozen snapshot keys. Loader errors fail the resolution of
// that span's content: silent degradation would turn evidence metrics into
// fiction.
func ResolveEvidenceSpans(spans []EvidenceSpan, load ChunkSource) ([]SpanResolution, error) {
	resolutions := make([]SpanResolution, 0, len(spans))
	for _, span := range spans {
		resolution := SpanResolution{
			ContentID:   span.ContentID,
			SpanStart:   span.SourceStart,
			SpanEnd:     span.SourceEnd,
			SnapshotKey: span.ChunkKey,
		}
		if span.SourceStart == 0 && span.SourceEnd == 0 {
			resolution.Degenerate = true
			resolutions = append(resolutions, resolution)
			continue
		}
		if span.SourceStart < 0 || span.SourceEnd < span.SourceStart {
			resolutions = append(resolutions, resolution)
			continue
		}
		chunks, err := load(span.ContentID)
		if err != nil {
			return nil, err
		}
		matched := ResolveSpanToChunks(span, chunks)
		for _, chunk := range matched {
			resolution.ResolvedKeys = append(resolution.ResolvedKeys, chunk.ChunkKey)
		}
		resolution.Resolved = len(matched) > 0
		if span.ChunkKey != "" {
			resolution.SnapshotDrift = !containsString(resolution.ResolvedKeys, span.ChunkKey)
		}
		resolutions = append(resolutions, resolution)
	}
	return resolutions, nil
}

// EvidenceContentHitAtK reports whether any span-resolved content id appears
// within the top-k retrieved ids: the span-grounded retrieval hit rate used
// by the body_evidence layer, where the answer is only obtainable from the
// annotated body region rather than the title.
func EvidenceContentHitAtK(ranked []int64, evidenceContentIDs map[int64]bool, k int) bool {
	limited := k
	if limited > len(ranked) {
		limited = len(ranked)
	}
	for i := 0; i < limited; i++ {
		if evidenceContentIDs[ranked[i]] {
			return true
		}
	}
	return false
}

// CitationWithinResolvedSpans checks one produced citation against the
// span-resolved chunk keys of its content: the answer-side half of H5. A
// citation whose chunk is outside every annotated span is still counted by
// content-level metrics but is not span-grounded evidence.
func CitationWithinResolvedSpans(contentID int64, chunkKey string, resolutions []SpanResolution) bool {
	for _, resolution := range resolutions {
		if resolution.ContentID != contentID {
			continue
		}
		if containsString(resolution.ResolvedKeys, chunkKey) {
			return true
		}
	}
	return false
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
