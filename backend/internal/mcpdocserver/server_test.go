package mcpdocserver

import (
	"context"
	"encoding/json"
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
	// 3) valid token from draft_read → applied
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
		"draft_id": 7, "confirm_token": read["confirm_token"], "user_confirmed": true, "edits": edits,
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

	// tags and title ops in one batch
	out, err := callTool(t, client, "draft_apply_edit", map[string]any{
		"draft_id": 7, "confirm_token": read["confirm_token"], "user_confirmed": true,
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

func TestConfirmTokenBinding(t *testing.T) {
	// token binds user+draft+timestamp: a different user's token never verifies
	draft := seedDraft(42)
	draft.UpdatedAt = time.Unix(1700000000, 0)
	token := ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix())
	if ConfirmToken([]byte("s"), 7, 43, draft.UpdatedAt.Unix()) == token {
		t.Fatal("token must bind the user")
	}
	if ConfirmToken([]byte("s"), 7, 42, draft.UpdatedAt.Unix()+1) == token {
		t.Fatal("token must bind the version timestamp")
	}
	if err := verifyConfirmToken([]byte("s"), "", draft); err != errConfirmMissing {
		t.Fatalf("empty token err = %v", err)
	}
	if err := verifyConfirmToken([]byte("s"), token, draft); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
}
