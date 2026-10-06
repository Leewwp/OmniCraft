package mcpdocserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"omnicraft/backend/internal/model"
)

// memStore is the in-memory DraftStore fake; ownership scoping is part of
// the contract under test.
type memStore struct {
	byID  map[int64]*model.AgentDraft
	owner int64
}

func (m *memStore) GetForUser(ctx context.Context, userID, draftID int64) (*model.AgentDraft, error) {
	d, ok := m.byID[draftID]
	if !ok || d.UserID != userID {
		return nil, errDraftNotFound
	}
	copy := *d
	return &copy, nil
}

func (m *memStore) Update(ctx context.Context, draft *model.AgentDraft) error {
	m.byID[draft.ID] = draft
	return nil
}

func callTool(t *testing.T, client *sdkmcp.ClientSession, name string, args map[string]any) (map[string]any, error) {
	t.Helper()
	res, err := client.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: name, Arguments: args,
	})
	if err != nil {
		return nil, err
	}
	if res.IsError {
		if len(res.Content) > 0 {
			if tc, ok := res.Content[0].(*sdkmcp.TextContent); ok {
				return nil, &toolError{text: tc.Text}
			}
		}
		return nil, &toolError{text: "tool error"}
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content shape: %#v", res.Content)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("tool result not JSON: %v (%s)", err, tc.Text)
	}
	return out, nil
}

type toolError struct{ text string }

func (e *toolError) Error() string { return e.text }

func newTestSession(t *testing.T, store *memStore, userID int64) *sdkmcp.ClientSession {
	t.Helper()
	server := NewServer(store, Options{UserID: userID, ConfirmSecret: []byte("test-secret")})
	t1, t2 := sdkmcp.NewInMemoryTransports()
	ss, err := server.Connect(context.Background(), t1, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "v0"}, nil).Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func seedDraft(userID int64) *model.AgentDraft {
	return &model.AgentDraft{
		ID: 7, UserID: userID, Title: "灯塔短篇", ContentType: "article",
		Body:   "# 灯塔短篇\n\n第一段：守塔人每天黄昏点灯。\n\n第二段：雾夜里来了一艘小船，桅杆上有一面反光的旗。",
		Status: "active",
	}
}

func TestDraftReadContract(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)

	// tools/list advertises the three tools
	list, err := client.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range list.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"draft_read", "draft_suggest", "draft_apply_edit"} {
		if !names[want] {
			t.Fatalf("tool %s missing from list: %v", want, names)
		}
	}

	out, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	if out["title"] != "灯塔短篇" {
		t.Fatalf("title = %v", out["title"])
	}
	paras, _ := out["paragraphs"].([]any)
	if len(paras) != 3 {
		t.Fatalf("paragraph count = %d, want 3", len(paras))
	}
	if token, _ := out["confirm_token"].(string); len(token) != 24 {
		t.Fatalf("confirm token shape = %v", out["confirm_token"])
	}

	// foreign draft = same not-found error (no ownership probing)
	if _, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 999}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unknown draft err = %v", err)
	}
}

func TestDraftSuggestContract(t *testing.T) {
	long := strings.Repeat("雾很大。", 100) // 500 runes > 300 threshold
	draft := seedDraft(42)
	draft.Body = "# 灯塔短篇\n\n" + long
	store := &memStore{byID: map[int64]*model.AgentDraft{7: draft}}
	client := newTestSession(t, store, 42)

	out, err := callTool(t, client, "draft_suggest", map[string]any{"draft_id": 7, "focus": "紧凑"})
	if err != nil {
		t.Fatal(err)
	}
	longs, _ := out["long_paragraphs"].([]any)
	if len(longs) != 1 {
		t.Fatalf("long paragraph flags = %d", len(longs))
	}
	meta, _ := out["suggested_metadata"].(map[string]any)
	if meta["title"] != "灯塔短篇" {
		t.Fatalf("heading-derived title = %v", meta["title"])
	}
	if _, ok := out["confirm_token"]; !ok {
		t.Fatal("suggest must return a confirm token")
	}
}

func TestDraftApplyEditConfirmGate(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)

	edits := []map[string]any{
		{"op": "replace_paragraph", "index": 2, "text": "第二段（更紧凑）：雾夜小船过，旗借灯光亮了一线。"},
	}
	// 1) no token → rejected, nothing written
	_, err := callTool(t, client, "draft_apply_edit", map[string]any{"draft_id": 7, "edits": edits})
	if err == nil || !strings.Contains(err.Error(), "confirmation token required") {
		t.Fatalf("missing token err = %v", err)
	}
	// 2) bogus token → rejected
	_, err = callTool(t, client, "draft_apply_edit", map[string]any{"draft_id": 7, "confirm_token": "deadbeef", "edits": edits})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("bogus token err = %v", err)
	}
	// 3) valid token from draft_read → stage 1 preview, then the batch
	//    token it issues carries user_confirmed=true into the write
	read, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	// stage 1 without user_confirmed: diff preview, no write
	stage1, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"], "edits": edits,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stage1["status"] != "needs_confirmation" {
		t.Fatalf("stage1 status = %v", stage1["status"])
	}
	if strings.Contains(store.byID[7].Body, "更紧凑") {
		t.Fatal("stage1 must not write")
	}

	out, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": stage1["confirm_token"], "user_confirmed": true, "edits": edits,
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied, _ := out["applied"].([]any); len(applied) != 1 {
		t.Fatalf("applied = %v", out["applied"])
	}
	if !strings.Contains(store.byID[7].Body, "更紧凑") {
		t.Fatalf("paragraph not replaced: %q", store.byID[7].Body)
	}
	// 4) the old token is now stale (updated_at advanced)
	_, err = callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"], "edits": edits,
	})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("replay must be rejected as stale: %v", err)
	}
}

func TestDraftApplyEditAtomicBatch(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)
	read, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	before := store.byID[7].Body

	// one valid edit + one out-of-range index: the whole batch rejects
	_, err = callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"],
		"edits": []map[string]any{
			{"op": "replace_paragraph", "index": 1, "text": "有效编辑"},
			{"op": "replace_paragraph", "index": 99, "text": "越界"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid edit batch") {
		t.Fatalf("atomic rejection err = %v", err)
	}
	if store.byID[7].Body != before {
		t.Fatal("a rejected batch must not partially apply")
	}

	// tags and title ops in one batch: stage 1 issues the batch token, then
	// the confirmed call applies all three ops atomically
	stage1, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"],
		"edits": []map[string]any{
			{"op": "update_title", "text": "灯塔短篇·改"},
			{"op": "update_tags", "tags": []string{"灯塔", "海"}},
			{"op": "insert_after", "index": -1, "text": "新开头段。"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stage1["status"] != "needs_confirmation" {
		t.Fatalf("stage1 status = %v", stage1["status"])
	}
	out, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": stage1["confirm_token"], "user_confirmed": true,
		"edits": []map[string]any{
			{"op": "update_title", "text": "灯塔短篇·改"},
			{"op": "update_tags", "tags": []string{"灯塔", "海"}},
			{"op": "insert_after", "index": -1, "text": "新开头段。"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if applied, _ := out["applied"].([]any); len(applied) != 3 {
		t.Fatalf("applied = %v", out["applied"])
	}
	if store.byID[7].Title != "灯塔短篇·改" {
		t.Fatalf("title = %s", store.byID[7].Title)
	}
}

// TestDraftApplyEditUserConfirmedCannotSkipPreview（审计 #9 / #814）：
// user_confirmed 是模型自报布尔——只有草案级令牌时它不得免预览直写，
// 必须先走 stage 1 拿到批次令牌。
func TestDraftApplyEditUserConfirmedCannotSkipPreview(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)
	before := store.byID[7].Body

	read, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	out, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"], "user_confirmed": true,
		"edits": []map[string]any{
			{"op": "replace_paragraph", "index": 1, "text": "未经预览的直写内容。"},
		},
	})
	if err != nil {
		t.Fatalf("draft-level token with user_confirmed must degrade to preview, got error %v", err)
	}
	if out["status"] != "needs_confirmation" {
		t.Fatalf("status = %v, want needs_confirmation", out["status"])
	}
	if store.byID[7].Body != before {
		t.Fatal("user_confirmed=true without a batch token must not write")
	}
}

// TestDraftApplyEditBatchTokenBinding（审计 #9 / #814）：预览批次 A 拿到的
// 确认令牌用于应用批次 B（换批次套用）必须失效。
func TestDraftApplyEditBatchTokenBinding(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)
	read, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	batchA := []map[string]any{
		{"op": "replace_paragraph", "index": 1, "text": "批次A的替换文本。"},
	}
	// stage 1 previews batch A and returns a batch-specific token.
	stage1, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"], "edits": batchA,
	})
	if err != nil {
		t.Fatal(err)
	}
	batchToken, _ := stage1["confirm_token"].(string)
	if batchToken == "" || batchToken == read["confirm_token"] {
		t.Fatalf("stage 1 must issue a batch-specific token, got %q", batchToken)
	}
	if strings.Contains(store.byID[7].Body, "批次A") {
		t.Fatal("stage 1 must not write")
	}

	// The batch-A token cannot apply a different batch B.
	batchB := []map[string]any{
		{"op": "replace_paragraph", "index": 1, "text": "批次B的偷换文本。"},
	}
	_, err = callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": batchToken, "user_confirmed": true, "edits": batchB,
	})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("batch-A token applying batch B must be rejected as stale, got %v", err)
	}
	if strings.Contains(store.byID[7].Body, "批次B") || strings.Contains(store.byID[7].Body, "批次A") {
		t.Fatalf("rejected batch-swap must not write, body = %q", store.byID[7].Body)
	}

	// The same token DOES apply its own batch.
	if _, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": batchToken, "user_confirmed": true, "edits": batchA,
	}); err != nil {
		t.Fatalf("batch token applying its own batch must succeed: %v", err)
	}
	if !strings.Contains(store.byID[7].Body, "批次A的替换文本") {
		t.Fatalf("batch A not applied: %q", store.byID[7].Body)
	}
}

// TestPreviewEditsTagsAndInsertDiff（审计 #9 / #814）：update_tags 与
// insert_after#-1 的预览 diff 必须带 before/after，预览才是可批准对象。
func TestPreviewEditsTagsAndInsertDiff(t *testing.T) {
	store := &memStore{byID: map[int64]*model.AgentDraft{7: seedDraft(42)}}
	client := newTestSession(t, store, 42)
	read, err := callTool(t, client, "draft_read", map[string]any{"draft_id": 7})
	if err != nil {
		t.Fatal(err)
	}
	out, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"],
		"edits": []map[string]any{
			{"op": "update_tags", "tags": []string{"灯塔", "海"}},
			{"op": "insert_after", "index": -1, "text": "新开头段。"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["status"] != "needs_confirmation" {
		t.Fatalf("status = %v", out["status"])
	}
	diff, _ := out["diff"].([]any)
	byOp := map[string]map[string]any{}
	for _, item := range diff {
		entry, _ := item.(map[string]any)
		if entry != nil {
			byOp[fmt.Sprint(entry["op"])] = entry
		}
	}
	tags, ok := byOp["update_tags"]
	if !ok {
		t.Fatalf("update_tags missing from diff: %v", diff)
	}
	if _, ok := tags["before"]; !ok {
		t.Fatalf("update_tags diff lacks before: %v", tags)
	}
	if after, _ := tags["after"].([]any); len(after) != 2 {
		t.Fatalf("update_tags diff after = %v", tags["after"])
	}
	insert, ok := byOp["insert_after#-1"]
	if !ok {
		t.Fatalf("insert_after#-1 missing from diff: %v", diff)
	}
	if after, _ := insert["after"].(string); after != "新开头段。" {
		t.Fatalf("insert_after#-1 diff after = %v", insert["after"])
	}
}

func TestConfirmTokenBinding(t *testing.T) {
	// token binds user+draft+timestamp+edit batch: a different user's,
	// version's or batch's token never verifies
	draft := seedDraft(42)
	draft.UpdatedAt = time.Unix(1700000000, 0)
	edits := []DraftEdit{{Op: "update_title", Text: "改"}}
	token, err := ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix(), batchHash(edits))
	if err != nil {
		t.Fatal(err)
	}
	if other, _ := ConfirmToken([]byte("s"), 7, 43, draft.UpdatedAt.Unix(), batchHash(edits)); other == token {
		t.Fatal("token must bind the user")
	}
	if other, _ := ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix()+1, batchHash(edits)); other == token {
		t.Fatal("token must bind the version timestamp")
	}
	if other, _ := ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix(), batchHash(nil)); other == token {
		t.Fatal("token must bind the edit batch")
	}
	swapped := []DraftEdit{{Op: "update_title", Text: "换"}}
	if other, _ := ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix(), batchHash(swapped)); other == token {
		t.Fatal("token must change when the batch changes")
	}

	kind, err := classifyConfirmToken([]byte("s"), "", draft, edits)
	if err != errConfirmMissing || kind != tokenMissing {
		t.Fatalf("empty token err = %v kind = %d", err, kind)
	}
	kind, err = classifyConfirmToken([]byte("s"), token, draft, edits)
	if err != nil || kind != tokenBatch {
		t.Fatalf("batch token classified %d err %v", kind, err)
	}
	kind, err = classifyConfirmToken([]byte("s"), token, draft, swapped)
	if err != errConfirmStale || kind != tokenStale {
		t.Fatalf("swapped batch must be stale, got kind %d err %v", kind, err)
	}
}

// TestConfirmTokenEmptySecret（审计 #9 / #814）：HMAC 空 secret 在 mint 与
// verify 调用处显式报错，而不是静默用可猜测密钥签发令牌。
func TestConfirmTokenEmptySecret(t *testing.T) {
	if _, err := ConfirmToken(nil, 7, 42, 0, ""); err == nil {
		t.Fatal("empty secret must be an explicit error at mint time")
	}
	draft := seedDraft(42)
	draft.UpdatedAt = time.Unix(1700000000, 0)
	if _, err := classifyConfirmToken(nil, "sometoken", draft, nil); err == nil {
		t.Fatal("empty secret must be an explicit error at verify time")
	}
}
