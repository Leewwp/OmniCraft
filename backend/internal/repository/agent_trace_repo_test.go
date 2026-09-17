package repository

import (
	"context"
	"reflect"
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

// seedCostLedgerFixture writes the SP-21 T7 matrix fixture: two days, two
// models, two conversations (plus one run without a conversation), a
// RUNNING node, a non-llm_round node and a NULL-token node that must all
// stay out of (or fold zero into) the token matrices. Seed instants sit at
// 12:00 UTC so day keys survive session-timezone rendering.
func seedCostLedgerFixture(t *testing.T, repo *AgentTraceRepository) {
	t.Helper()
	day1 := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)

	runs := []model.AgentTraceRun{
		{TraceID: "t-a", Status: model.AgentTraceStatusSuccess, StartedAt: day1, ConversationID: int64PtrRepo(42)},
		{TraceID: "t-b", Status: model.AgentTraceStatusSuccess, StartedAt: day2, ConversationID: int64PtrRepo(42)},
		{TraceID: "t-c", Status: model.AgentTraceStatusSuccess, StartedAt: day1, ConversationID: int64PtrRepo(43)},
		{TraceID: "t-d", Status: model.AgentTraceStatusSuccess, StartedAt: day2},
	}
	if err := repo.UpsertRuns(context.Background(), runs); err != nil {
		t.Fatal(err)
	}

	i1000, o200 := int64(1000), int64(200)
	i500, o100 := int64(500), int64(100)
	i300, o60 := int64(300), int64(60)
	i200, o40 := int64(200), int64(40)
	i700, o140 := int64(700), int64(140)
	i999, o999 := int64(999), int64(999)
	nodes := []model.AgentTraceNode{
		// Mixed-case model name on one minimax node: buckets are
		// case-normalized, so it must merge into "minimax-m3".
		{TraceID: "t-a", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "MiniMax-M3", TokensIn: &i1000, TokensOut: &o200},
		{TraceID: "t-a", NodeKey: "llm_round_2", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "deepseek-chat", TokensIn: &i500, TokensOut: &o100},
		{TraceID: "t-b", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day2, Model: "minimax-m3", TokensIn: &i300, TokensOut: &o60},
		{TraceID: "t-c", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "minimax-m3", TokensIn: &i200, TokensOut: &o40},
		{TraceID: "t-d", NodeKey: "llm_round_1", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day2, Model: "deepseek-chat", TokensIn: &i700, TokensOut: &o140},
		// RUNNING round: tokens only land at End(), so it must be excluded.
		{TraceID: "t-b", NodeKey: "llm_round_2", NodeType: "llm_round", Status: model.AgentTraceStatusRunning, StartedAt: day2, Model: "minimax-m3", TokensIn: &i999, TokensOut: &o999},
		// Non-llm_round node type: never part of the cost ledger.
		{TraceID: "t-c", NodeKey: "tool_search_0", NodeType: "tool", Status: model.AgentTraceStatusSuccess, StartedAt: day1, Model: "minimax-m3", TokensIn: &i999, TokensOut: &o999},
		// Terminal round with NULL usage folds zeros into its (day, model) cell.
		{TraceID: "t-d", NodeKey: "llm_round_2", NodeType: "llm_round", Status: model.AgentTraceStatusSuccess, StartedAt: day2, Model: "deepseek-chat"},
	}
	if err := repo.UpsertNodes(context.Background(), nodes); err != nil {
		t.Fatal(err)
	}
}

func TestAgentTraceRepoLLMCostMatrices(t *testing.T) {
	repo := setupTraceRepo(t)
	ctx := context.Background()
	seedCostLedgerFixture(t, repo)

	dayModel, err := repo.AggregateLLMCostsByDayModel(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantDayModel := []LLMCostDayModel{
		{Day: "2026-09-16", Model: "deepseek-chat", TokensIn: 500, TokensOut: 100},
		{Day: "2026-09-16", Model: "minimax-m3", TokensIn: 1200, TokensOut: 240},
		{Day: "2026-09-17", Model: "deepseek-chat", TokensIn: 700, TokensOut: 140},
		{Day: "2026-09-17", Model: "minimax-m3", TokensIn: 300, TokensOut: 60},
	}
	if !reflect.DeepEqual(dayModel, wantDayModel) {
		t.Fatalf("day x model matrix = %+v, want %+v", dayModel, wantDayModel)
	}

	convModel, err := repo.AggregateLLMCostsByConversationModel(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantConvModel := []LLMCostConversationModel{
		{ConversationID: 42, Model: "deepseek-chat", TokensIn: 500, TokensOut: 100},
		{ConversationID: 42, Model: "minimax-m3", TokensIn: 1300, TokensOut: 260},
		{ConversationID: 43, Model: "minimax-m3", TokensIn: 200, TokensOut: 40},
	}
	if !reflect.DeepEqual(convModel, wantConvModel) {
		t.Fatalf("conversation x model matrix = %+v, want %+v", convModel, wantConvModel)
	}

	turns, err := repo.CountRunsByConversationIDs(ctx, []int64{42, 43}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if turns[42] != 2 || turns[43] != 1 || len(turns) != 2 {
		t.Fatalf("turns = %v, want 42:2 43:1", turns)
	}

	// Window scoping: only day-2 cells survive a from bound on day 2.
	day2Start := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	dayModel, err = repo.AggregateLLMCostsByDayModel(ctx, &day2Start, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(dayModel) != 2 || dayModel[0].Day != "2026-09-17" || dayModel[0].Model != "deepseek-chat" {
		t.Fatalf("windowed day x model matrix = %+v, want only the two 2026-09-17 cells", dayModel)
	}

	if empty, err := repo.CountRunsByConversationIDs(ctx, nil, nil, nil); err != nil || len(empty) != 0 {
		t.Fatalf("empty id list should short-circuit: %v %v", empty, err)
	}
}

func int64PtrRepo(v int64) *int64 { return &v }
func intPtrRepo(v int) *int       { return &v }
