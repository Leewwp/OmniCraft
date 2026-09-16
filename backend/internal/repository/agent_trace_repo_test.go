package repository

import (
	"context"
	"testing"
	"time"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/testutil"
)

func setupTraceRepo(t *testing.T) *AgentTraceRepository {
	t.Helper()
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, "../../migrations/081_agent_trace.sql")
	return NewAgentTraceRepository(db)
}

func TestAgentTraceRepoUpsertRunLifecycle(t *testing.T) {
	repo := setupTraceRepo(t)
	ctx := context.Background()
	start := time.Now().Add(-2 * time.Second)

	if err := repo.UpsertRuns(ctx, []model.AgentTraceRun{{
		TraceID:        "run-1",
		Status:         model.AgentTraceStatusRunning,
		StartedAt:      start,
		UserID:         int64PtrRepo(7),
		Surface:        "content",
		PromptName:     "agent_system",
		PromptVersion:  intPtrRepo(1),
		ConversationID: int64PtrRepo(42),
	}}); err != nil {
		t.Fatal(err)
	}

	end := time.Now()
	dur, ttft := int64(2100), int64(650)
	msg := int64(99)
	// The repo upsert is deliberately "latest complete row wins": merging a
	// late terminal fragment onto the flushed start state is the writer's
	// contract (see TestWriterTerminalUpdateAfterFlush), so this terminal
	// record repeats the start identity exactly as the writer would send it.
	if err := repo.UpsertRuns(ctx, []model.AgentTraceRun{{
		TraceID:        "run-1",
		Status:         model.AgentTraceStatusSuccess,
		StartedAt:      start,
		UserID:         int64PtrRepo(7),
		Surface:        "content",
		PromptName:     "agent_system",
		PromptVersion:  intPtrRepo(1),
		ConversationID: int64PtrRepo(42),
		EndedAt:        &end,
		DurationMs:     &dur,
		TTFTMs:         &ttft,
		Model:          "minimax-m3",
		AnswerKind:     "grounded_content",
		MessageID:      &msg,
		RoutingEvents:  model.JSONB(`[{"from":"minimax-m3","to":"deepseek-chat","reason":"provider_error"}]`),
	}}); err != nil {
		t.Fatal(err)
	}

	var got model.AgentTraceRun
	if err := repo.db.WithContext(ctx).Where("trace_id = ?", "run-1").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != model.AgentTraceStatusSuccess || got.Model != "minimax-m3" {
		t.Fatalf("terminal upsert lost fields: %+v", got)
	}
	// Start identity must survive the terminal update arm.
	if got.UserID == nil || *got.UserID != 7 || got.Surface != "content" || got.PromptName != "agent_system" {
		t.Fatalf("start identity clobbered by terminal update: %+v", got)
	}
	if got.TTFTMs == nil || *got.TTFTMs != 650 || got.MessageID == nil || *got.MessageID != 99 {
		t.Fatalf("terminal fields missing: %+v", got)
	}
}

func TestAgentTraceRepoUpsertNodeLifecycle(t *testing.T) {
	repo := setupTraceRepo(t)
	ctx := context.Background()

	if err := repo.UpsertNodes(ctx, []model.AgentTraceNode{{
		TraceID:   "run-1",
		NodeKey:   "llm_round_1",
		NodeType:  "llm",
		Status:    model.AgentTraceStatusRunning,
		StartedAt: time.Now().Add(-time.Second),
	}}); err != nil {
		t.Fatal(err)
	}

	end := time.Now()
	dur, tin, tout := int64(900), int64(1200), int64(350)
	cost := 0.0042
	parent := "root"
	if err := repo.UpsertNodes(ctx, []model.AgentTraceNode{{
		TraceID:          "run-1",
		NodeKey:          "llm_round_1",
		Status:           model.AgentTraceStatusSuccess,
		EndedAt:          &end,
		DurationMs:       &dur,
		Model:            "minimax-m3",
		PromptDigest:     "digest",
		TokensIn:         &tin,
		TokensOut:        &tout,
		CostEstimate:     &cost,
		ParentNodeKey:    &parent,
		Depth:            1,
	}}); err != nil {
		t.Fatal(err)
	}

	var got model.AgentTraceNode
	if err := repo.db.WithContext(ctx).Where("trace_id = ? AND node_key = ?", "run-1", "llm_round_1").First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != model.AgentTraceStatusSuccess || got.CostEstimate == nil || *got.CostEstimate != 0.0042 {
		t.Fatalf("node terminal upsert lost fields: %+v", got)
	}
	if got.ParentNodeKey == nil || *got.ParentNodeKey != "root" || got.Depth != 1 {
		t.Fatalf("node tree fields missing: %+v", got)
	}
}

func TestAgentTraceRepoPurgeBefore(t *testing.T) {
	repo := setupTraceRepo(t)
	ctx := context.Background()

	old := time.Now().Add(-48 * time.Hour)
	if err := repo.UpsertRuns(ctx, []model.AgentTraceRun{{TraceID: "old-run", Status: model.AgentTraceStatusSuccess, StartedAt: old}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertRuns(ctx, []model.AgentTraceRun{{TraceID: "new-run", Status: model.AgentTraceStatusSuccess, StartedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	// Backdate the old row's created_at past the retention cutoff.
	if err := repo.db.WithContext(ctx).Exec(`UPDATE agent_trace_runs SET created_at = NOW() - INTERVAL '40 days' WHERE trace_id = 'old-run'`).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertNodes(ctx, []model.AgentTraceNode{{TraceID: "old-run", NodeKey: "n1", NodeType: "llm", Status: model.AgentTraceStatusSuccess, StartedAt: old}}); err != nil {
		t.Fatal(err)
	}
	if err := repo.db.WithContext(ctx).Exec(`UPDATE agent_trace_nodes SET created_at = NOW() - INTERVAL '40 days' WHERE trace_id = 'old-run'`).Error; err != nil {
		t.Fatal(err)
	}

	removed, err := repo.PurgeBefore(ctx, time.Now().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2 (one run + one node)", removed)
	}
	var remain int64
	if err := repo.db.WithContext(ctx).Model(&model.AgentTraceRun{}).Count(&remain).Error; err != nil {
		t.Fatal(err)
	}
	if remain != 1 {
		t.Fatalf("remaining runs = %d, want 1 (new-run)", remain)
	}
}

func int64PtrRepo(v int64) *int64 { return &v }
func intPtrRepo(v int) *int       { return &v }
