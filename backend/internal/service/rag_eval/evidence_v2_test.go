package rageval

import (
	"errors"
	"testing"

	"omnicraft/backend/internal/model"
)

func chunk(contentID int64, index, start, end int, key string) model.RagChunk {
	return model.RagChunk{ContentID: contentID, ChunkIndex: index, ChunkKey: key, SourceStart: start, SourceEnd: end}
}

// H5: spans resolve onto the chunks of the current chunking_version by
// half-open overlap, not by the frozen chunk_key snapshot.
func TestResolveSpanToChunks(t *testing.T) {
	chunks := []model.RagChunk{
		chunk(10, 0, 0, 100, "c0"),
		chunk(10, 1, 100, 200, "c1"),
		chunk(10, 2, 200, 300, "c2"),
		chunk(11, 0, 0, 50, "other"),
	}

	// span inside chunk 1 only
	got := ResolveSpanToChunks(EvidenceSpan{ContentID: 10, SourceStart: 120, SourceEnd: 150}, chunks)
	if len(got) != 1 || got[0].ChunkKey != "c1" {
		t.Fatalf("mid-chunk span = %v, want [c1]", got)
	}

	// span crossing the c1/c2 boundary hits both chunks
	got = ResolveSpanToChunks(EvidenceSpan{ContentID: 10, SourceStart: 150, SourceEnd: 250}, chunks)
	if len(got) != 2 || got[0].ChunkKey != "c1" || got[1].ChunkKey != "c2" {
		t.Fatalf("boundary span = %v, want [c1 c2]", got)
	}

	// boundary-touching (end == next start) is not an overlap: half-open
	got = ResolveSpanToChunks(EvidenceSpan{ContentID: 10, SourceStart: 0, SourceEnd: 100}, chunks)
	if len(got) != 1 || got[0].ChunkKey != "c0" {
		t.Fatalf("span [0,100) = %v, want [c0] only", got)
	}

	// content isolation
	got = ResolveSpanToChunks(EvidenceSpan{ContentID: 11, SourceStart: 10, SourceEnd: 20}, chunks)
	if len(got) != 1 || got[0].ChunkKey != "other" {
		t.Fatalf("content 11 span = %v", got)
	}

	// degenerate and inverted spans never match
	if got := ResolveSpanToChunks(EvidenceSpan{ContentID: 10, SourceStart: 0, SourceEnd: 0}, chunks); len(got) != 0 {
		t.Fatalf("zero-length span = %v, want none", got)
	}
	if got := ResolveSpanToChunks(EvidenceSpan{ContentID: 10, SourceStart: 50, SourceEnd: 10}, chunks); len(got) != 0 {
		t.Fatalf("inverted span = %v, want none", got)
	}
}

func TestResolveEvidenceSpans(t *testing.T) {
	chunksByContent := map[int64][]model.RagChunk{
		10: {chunk(10, 0, 0, 100, "old-key"), chunk(10, 1, 100, 200, "new-key")},
	}
	load := func(contentID int64) ([]model.RagChunk, error) { return chunksByContent[contentID], nil }

	spans := []EvidenceSpan{
		{ContentID: 10, SourceStart: 10, SourceEnd: 20, ChunkKey: "old-key", ChunkingVersion: 1},
		{ContentID: 10, SourceStart: 0, SourceEnd: 0}, // v1 placeholder
		{ContentID: 99, SourceStart: 0, SourceEnd: 10},
	}
	resolutions, err := ResolveEvidenceSpans(spans, load)
	if err != nil {
		t.Fatalf("resolve spans: %v", err)
	}
	if len(resolutions) != 3 {
		t.Fatalf("resolutions = %d, want 3", len(resolutions))
	}

	// span 0 resolves into chunk 0 whose key matches the snapshot: no drift
	r := resolutions[0]
	if !r.Resolved || r.SnapshotDrift || len(r.ResolvedKeys) != 1 || r.ResolvedKeys[0] != "old-key" {
		t.Fatalf("span 0 resolution = %+v", r)
	}

	// the (0,0) placeholder is marked degenerate and skipped
	if !resolutions[1].Degenerate || resolutions[1].Resolved {
		t.Fatalf("placeholder resolution = %+v", resolutions[1])
	}

	// unknown content resolves to nothing without error
	if resolutions[2].Resolved || resolutions[2].Degenerate {
		t.Fatalf("unknown content resolution = %+v", resolutions[2])
	}

	// re-chunked content: span lands in the chunk the snapshot no longer
	// names — resolution succeeds, drift is reported (snapshot is a
	// cross-check, never truth).
	spans = []EvidenceSpan{{ContentID: 10, SourceStart: 120, SourceEnd: 130, ChunkKey: "old-key"}}
	resolutions, err = ResolveEvidenceSpans(spans, load)
	if err != nil {
		t.Fatalf("resolve drifted span: %v", err)
	}
	if !resolutions[0].Resolved || !resolutions[0].SnapshotDrift {
		t.Fatalf("drifted resolution = %+v", resolutions[0])
	}

	// loader errors fail loudly
	failing := func(int64) ([]model.RagChunk, error) { return nil, errors.New("db down") }
	if _, err := ResolveEvidenceSpans(spans, failing); err == nil {
		t.Fatal("loader error must propagate")
	}
}

func TestEvidenceContentHitAtK(t *testing.T) {
	evidence := map[int64]bool{7: true}
	if !EvidenceContentHitAtK([]int64{3, 7}, evidence, 10) {
		t.Fatal("evidence id in top-10 must hit")
	}
	if EvidenceContentHitAtK([]int64{3, 7}, evidence, 1) {
		t.Fatal("evidence id outside top-k must miss")
	}
	if EvidenceContentHitAtK([]int64{1, 2}, evidence, 10) {
		t.Fatal("no evidence id retrieved must miss")
	}
}

func TestCitationWithinResolvedSpans(t *testing.T) {
	resolutions := []SpanResolution{
		{ContentID: 10, ResolvedKeys: []string{"c0", "c1"}},
		{ContentID: 11, ResolvedKeys: []string{"d0"}},
	}
	if !CitationWithinResolvedSpans(10, "c1", resolutions) {
		t.Fatal("citation inside the span-resolved chunks must match")
	}
	if CitationWithinResolvedSpans(10, "c9", resolutions) {
		t.Fatal("citation outside every resolved chunk must not match")
	}
	if CitationWithinResolvedSpans(12, "c0", resolutions) {
		t.Fatal("other content must not match")
	}
}
