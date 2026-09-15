package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
)

// searchIPsFixture wires a fake ipSearch seam (the production closure hits
// PostgreSQL-only tsvector SQL) plus the config allowlist.
func searchIPsFixture(t *testing.T, db *gorm.DB, ips []model.IP, allow []string) *AgentService {
	t.Helper()
	provider := &recordingToolProvider{}
	svc := NewAgentService(
		provider,
		nil,
		nil,
		nil,
		db,
		&config.Config{
			Agent: config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 2, CitationMaxCount: 5, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200},
			RAG:   config.RAGConfig{Hybrid: config.RAGHybridConfig{FinalTopK: 10}},
			IPCategories: allow,
		},
	)
	svc.ipSearch = func(_ context.Context, query, category string, limit int) ([]model.IP, error) {
		if query == "boom" {
			return nil, errors.New("db down")
		}
		if limit != 10 {
			return nil, errors.New("unexpected limit")
		}
		var out []model.IP
		for _, ip := range ips {
			if category != "" && ip.Category != category {
				continue
			}
			out = append(out, ip)
		}
		return out, nil
	}
	return svc
}

func TestToolSearchIPsArgValidation(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, nil, []string{"game", "anime"})

	cases := []struct {
		name string
		args string
		want error
	}{
		{"unknown field", `{"query":"x","filter":"game"}`, ErrAgentToolInvalidArgs},
		{"empty query", `{"query":"  "}`, ErrAgentToolInvalidArgs},
		{"oversize query", `{"query":"` + strings.Repeat("字", defaultMaxToolQueryLength+1) + `"}`, ErrAgentToolInvalidArgs},
		{"illegal category", `{"query":"x","category":"gaming"}`, ErrAgentToolInvalidArgs},
		{"legal category", `{"query":"x","category":"game"}`, nil},
		{"no category", `{"query":"x"}`, nil},
	}
	for _, tc := range cases {
		_, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(tc.args), 1, nil)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err=%v want=%v", tc.name, err, tc.want)
		}
	}
	if _, err := svc.ExecuteTool(context.Background(), "search_ipes", json.RawMessage(`{"query":"x"}`), 1, nil); !errors.Is(err, ErrAgentToolUnknown) {
		t.Errorf("typo'd tool name should stay unknown, got %v", err)
	}
}

func TestToolSearchIPsOutcomeMapping(t *testing.T) {
	db := seedAgentGroundingDB(t)
	long := strings.Repeat("描", 200)
	svc := searchIPsFixture(t, db, []model.IP{
		{ID: 7, Name: "苍穹档案", Slug: "vault-sky", Category: "game", Description: long},
		{ID: 0, Name: "无 id 垃圾行", Category: "game"},
		{ID: 9, Name: "", Category: "game"},
	}, []string{"game"})

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"query":"苍穹"}`), 1, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(outcome.IPs) != 3 {
		t.Fatalf("seam results = %d, want 3 (mapping happens after)", len(outcome.IPs))
	}
	// The seam returned raw rows including junk; the summary keeps them as-is
	// (server-owned), but the citation shaper below must drop invalid ones.
	citation, ok := citationFromIPSummary(outcome.IPs[0])
	if !ok {
		t.Fatalf("valid ip summary must yield a citation")
	}
	if citation.Zone != "ip" || citation.Route != "/ip/7" || citation.Title != "苍穹档案" || citation.Category != "game" {
		t.Errorf("citation shape wrong: %+v", citation)
	}
	if got := len([]rune(citation.Excerpt)); got > 160 {
		t.Errorf("excerpt runes = %d, want <=160", got)
	}
	if _, ok := citationFromIPSummary(outcome.IPs[1]); ok {
		t.Errorf("zero-id summary must not yield a citation")
	}
	if _, ok := citationFromIPSummary(outcome.IPs[2]); ok {
		t.Errorf("empty-name summary must not yield a citation")
	}
	if outcome.Execution.Status != AgentToolStatusSuccess {
		t.Errorf("execution status = %s", outcome.Execution.Status)
	}
	if agentToolHitCount(outcome) != 3 {
		t.Errorf("hit count = %d, want 3", agentToolHitCount(outcome))
	}

	if _, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"query":"boom"}`), 1, nil); err == nil {
		t.Errorf("seam error must propagate as tool error")
	}
}

func TestSearchIPsCategoryFilterPassedToSeam(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, []model.IP{
		{ID: 7, Name: "苍穹档案", Category: "game"},
		{ID: 8, Name: "银杏电台", Category: "anime"},
	}, []string{"game", "anime"})

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"query":"档案","category":"anime"}`), 1, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(outcome.IPs) != 1 || outcome.IPs[0].ID != 8 {
		t.Fatalf("category filter must reach the seam, got %+v", outcome.IPs)
	}
}

func TestIPCitationRevalidation(t *testing.T) {
	db := seedAgentGroundingDB(t)
	now := model.IP{ID: 300, Name: "苍穹档案", Slug: "vault-sky", Category: "game", Status: "approved"}
	pending := model.IP{ID: 301, Name: "未过审设定", Slug: "pending", Category: "game", Status: "pending"}
	if err := db.Create(&[]model.IP{now, pending}).Error; err != nil {
		t.Fatalf("seed ips: %v", err)
	}
	_ = pending
	svc := searchIPsFixture(t, db, nil, []string{"game"})

	cases := []struct {
		name     string
		citation AgentCitation
		want     string
	}{
		{"approved ip passes", AgentCitation{ContentID: 300, Title: "苍穹档案", Zone: "ip", Route: "/ip/300"}, ""},
		{"pending ip rejected", AgentCitation{ContentID: 301, Title: "未过审设定", Zone: "ip", Route: "/ip/301"}, "not_visible"},
		{"missing ip rejected", AgentCitation{ContentID: 404, Title: "不存在", Zone: "ip", Route: "/ip/404"}, "not_visible"},
		{"route mismatch rejected", AgentCitation{ContentID: 300, Title: "苍穹档案", Zone: "ip", Route: "/original/300"}, "invalid_route"},
		{"title mismatch rejected", AgentCitation{ContentID: 300, Title: "改名后的档案", Zone: "ip", Route: "/ip/300"}, "ip_metadata_mismatch"},
		{"missing fields rejected", AgentCitation{ContentID: 300, Zone: "ip"}, "missing_fields"},
	}
	for _, tc := range cases {
		if got := svc.citationRejectionReason(context.Background(), 1, tc.citation); got != tc.want {
			t.Errorf("%s: reason=%q want=%q", tc.name, got, tc.want)
		}
	}
}

func TestIPCitationMarshalShape(t *testing.T) {
	raw, err := json.Marshal(AgentCitation{ContentID: 7, Title: "苍穹档案", Zone: "ip", Route: "/ip/7", Excerpt: "简介", Category: "game"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"content_version", "chunk_key", "chunk_index", "source"} {
		if _, exists := decoded[key]; exists {
			t.Errorf("ip citation must not carry %q, got %s", key, raw)
		}
	}
	if decoded["zone"] != "ip" || decoded["route"] != "/ip/7" || decoded["category"] != "game" {
		t.Errorf("ip citation shape wrong: %s", raw)
	}
}

func TestSearchIPsToolDefinitionCategoryEnum(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, nil, []string{"game", "anime", "other"})
	var found bool
	for _, def := range svc.ToolDefinitions() {
		if def.Name != ToolSearchIPs {
			continue
		}
		found = true
		props := def.Parameters["properties"].(map[string]interface{})
		cat := props["category"].(map[string]interface{})
		desc := cat["description"].(string)
		if !strings.Contains(desc, "game, anime, other") {
			t.Errorf("category description must list the allowlist slugs, got %q", desc)
		}
	}
	if !found {
		t.Fatalf("search_ips missing from ToolDefinitions: %+v", svc.ToolDefinitions())
	}
	if names := svc.RegisteredToolNames(); !contains(names, ToolSearchIPs) {
		t.Errorf("search_ips missing from RegisteredToolNames: %v", names)
	}
}

func TestSystemPromptListsIPCategorySlugs(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, nil, []string{"game", "vtuber"})
	prompt := svc.serverOwnedSystemPrompt(model.AgentChatSurfaceGlobal, nil)
	if !strings.Contains(prompt.Content, "search_ips") {
		t.Errorf("prompt must mention search_ips, got: %s", prompt.Content)
	}
	if !strings.Contains(prompt.Content, "game, vtuber") {
		t.Errorf("prompt must list legal category slugs, got: %s", prompt.Content)
	}
}

func TestDeriveToolArgsSummarySearchIPs(t *testing.T) {
	summary, err := deriveToolArgsSummary(ToolSearchIPs, json.RawMessage(`{"query":"苍穹 档案","category":"game"}`))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !strings.Contains(summary, "苍穹 档案") || !strings.Contains(summary, "game") {
		t.Errorf("summary should carry query and category, got %q", summary)
	}
	empty, err := deriveToolArgsSummary(ToolSearchIPs, json.RawMessage(`{"query":"  "}`))
	if err != nil || empty != "" {
		t.Errorf("empty query summary should be empty string, got %q err=%v", empty, err)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
