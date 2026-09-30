package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"omnicraft/backend/internal/pkg/llm"

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
			Agent:        config.AgentConfig{WebAgentEnabled: true, MaxToolCallsPerTurn: 2, CitationMaxCount: 5, MaxUserMessageChars: 4000, ChatMaxContextMsgs: 10, MaxOutputTokens: 1200},
			RAG:          config.RAGConfig{Hybrid: config.RAGHybridConfig{FinalTopK: 10}},
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
	prompt := svc.serverOwnedSystemPrompt(context.Background(), model.AgentChatSurfaceGlobal, nil)
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

func TestSearchIPsCategoryBrowseFallback(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, []model.IP{
		{ID: 7, Name: "苍穹档案", Category: "game"},
	}, []string{"game"})
	// 泛词 query（如「游戏」）在具体 IP 名/简介全文匹配不到 → 空结果；
	// 带 category 时应回退为纯分类浏览（空 query），拿到分类清单。
	var calls [][2]string
	svc.ipSearch = func(_ context.Context, query, category string, limit int) ([]model.IP, error) {
		calls = append(calls, [2]string{query, category})
		if query != "" {
			return nil, nil
		}
		return []model.IP{{ID: 7, Name: "苍穹档案", Category: "game"}}, nil
	}
	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"query":"游戏","category":"game"}`), 1, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(outcome.IPs) != 1 || outcome.IPs[0].ID != 7 {
		t.Fatalf("fallback must surface category browse results, got %+v", outcome.IPs)
	}
	if len(calls) != 2 || calls[0][0] != "游戏" || calls[1][0] != "" {
		t.Fatalf("expected keyword attempt then empty-query browse retry, got %v", calls)
	}
	// 无 category 时不回退：一次调用、空结果照实返回。
	calls = nil
	svc.ipSearch = func(_ context.Context, query, category string, limit int) ([]model.IP, error) {
		calls = append(calls, [2]string{query, category})
		return nil, nil
	}
	outcome, err = svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"query":"游戏"}`), 1, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(outcome.IPs) != 0 || len(calls) != 1 {
		t.Fatalf("no category = no retry, got ips=%d calls=%v", len(outcome.IPs), calls)
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

// ---------------------------------------------------------------------------
// #754 A：search_ips 显式浏览通道（sort=newest/most_contents + 空 query）
// ---------------------------------------------------------------------------

// browseIPsFixture 在 searchIPsFixture 之上挂 fake ipBrowse seam，并记录
// 调用参数（sort/category/limit 透传断言）。
func browseIPsFixture(t *testing.T, db *gorm.DB, ips []model.IP, allow []string) (*AgentService, *[]string) {
	t.Helper()
	svc := searchIPsFixture(t, db, ips, allow)
	calls := &[]string{}
	svc.ipBrowse = func(_ context.Context, sort, category string, limit int) ([]model.IP, error) {
		*calls = append(*calls, fmt.Sprintf("%s|%s|%d", sort, category, limit))
		if sort == "browse-boom" {
			return nil, errors.New("db down")
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
	return svc, calls
}

func TestToolSearchIPsExplicitBrowseParamTable(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc, calls := browseIPsFixture(t, db, []model.IP{{ID: 7, Name: "苍穹档案", Category: "game"}}, []string{"game", "anime"})

	type row struct {
		name       string
		args       string
		wantErr    error
		wantBrowse string // 期望的 browse seam 调用记录；空 = 不应触达
	}
	cases := []row{
		// 未传 sort、非空 query → 原关键词检索（seam 不触达 browse）。
		{"keyword only", `{"query":"苍穹"}`, nil, ""},
		{"keyword + category", `{"query":"苍穹","category":"game"}`, nil, ""},
		// 未传 sort、query 空/缺失 → invalid_args（不悄悄全库浏览）。
		{"no sort empty query", `{"query":"  "}`, ErrAgentToolInvalidArgs, ""},
		{"no sort missing query", `{}`, ErrAgentToolInvalidArgs, ""},
		// sort + 空 query → 显式浏览；category 合法可选。
		{"browse newest", `{"sort":"newest"}`, nil, "newest||10"},
		{"browse most_contents", `{"sort":"most_contents"}`, nil, "most_contents||10"},
		{"browse newest + category", `{"sort":"newest","category":"game"}`, nil, "newest|game|10"},
		{"browse most_contents + category", `{"sort":"most_contents","category":"anime"}`, nil, "most_contents|anime|10"},
		{"browse query whitespace", `{"sort":"newest","query":"   "}`, nil, "newest||10"},
		// sort 非法/显式空串、或 sort 配非空 query → invalid_args，不静默丢弃条件。
		{"sort invalid value", `{"sort":"hot"}`, ErrAgentToolInvalidArgs, ""},
		{"sort invalid name order", `{"sort":"name"}`, ErrAgentToolInvalidArgs, ""},
		{"sort explicit empty string", `{"sort":""}`, ErrAgentToolInvalidArgs, ""},
		{"sort explicit whitespace", `{"sort":"  "}`, ErrAgentToolInvalidArgs, ""},
		{"sort empty string with query", `{"sort":"","query":"苍穹"}`, ErrAgentToolInvalidArgs, ""},
		{"sort with non-empty query", `{"sort":"newest","query":"苍穹"}`, ErrAgentToolInvalidArgs, ""},
		{"sort with illegal category", `{"sort":"newest","category":"gaming"}`, ErrAgentToolInvalidArgs, ""},
	}
	for _, tc := range cases {
		*calls = (*calls)[:0]
		outcome, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(tc.args), 1, nil)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err=%v want=%v", tc.name, err, tc.wantErr)
			continue
		}
		if tc.wantBrowse == "" {
			if len(*calls) != 0 {
				t.Errorf("%s: browse seam must not be touched, got %v", tc.name, *calls)
			}
			continue
		}
		if len(*calls) != 1 || (*calls)[0] != tc.wantBrowse {
			t.Errorf("%s: browse seam calls = %v, want [%s]", tc.name, *calls, tc.wantBrowse)
		}
		if err == nil && outcome != nil && outcome.Execution.Status != AgentToolStatusSuccess {
			t.Errorf("%s: browse must stay a success-shaped outcome, got %s", tc.name, outcome.Execution.Status)
		}
	}
}

// 浏览空结果保留 ok=true / success / hits=0（#754 B 的触发面），错误沿 tool
// error 路径不伪装成空结果；browse seam 缺席时显式报错。
func TestToolSearchIPsBrowseEmptyAndUnavailable(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc, _ := browseIPsFixture(t, db, nil, []string{"game"})

	outcome, err := svc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"sort":"newest"}`), 1, nil)
	if err != nil {
		t.Fatalf("empty browse is a success, got %v", err)
	}
	if len(outcome.IPs) != 0 || outcome.Execution.Status != AgentToolStatusSuccess || agentToolHitCount(outcome) != 0 {
		t.Fatalf("empty browse must keep ok/success/hits=0 shape, got %+v", outcome)
	}

	errorSvc, _ := browseIPsFixture(t, db, nil, []string{"game"})
	// 让 fake browse 对 most_contents 抛错（fake 以 sort=browse-boom 判错，
	// 这里直接换成显式错误 seam）。
	errorSvc.ipBrowse = func(context.Context, string, string, int) ([]model.IP, error) {
		return nil, errors.New("browse down")
	}
	if _, err := errorSvc.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"sort":"most_contents"}`), 1, nil); err == nil {
		t.Fatal("browse seam error must propagate as a tool error, not an empty success")
	}

	noBrowse := searchIPsFixture(t, db, nil, []string{"game"})
	noBrowse.ipBrowse = nil
	if _, err := noBrowse.ExecuteTool(context.Background(), ToolSearchIPs, json.RawMessage(`{"sort":"newest"}`), 1, nil); err == nil || !strings.Contains(err.Error(), "ip browse unavailable") {
		t.Fatalf("unwired browse seam must fail explicitly, got %v", err)
	}
}

// ArgsSummary：浏览模式以 browse:<sort>（·category）呈现，空 query 不再让
// 摘要为空；关键词模式维持原样。
func TestDeriveToolArgsSummarySearchIPsBrowse(t *testing.T) {
	cases := []struct {
		args string
		want string
	}{
		{`{"sort":"newest"}`, "browse:newest"},
		{`{"sort":"most_contents","category":"game"}`, "browse:most_contents · game"},
		{`{"sort":"newest","query":""}`, "browse:newest"},
		{`{"query":"苍穹"}`, "苍穹"},
		{`{"query":"苍穹","category":"game"}`, "苍穹 · game"},
	}
	for _, tc := range cases {
		got, err := deriveToolArgsSummary(ToolSearchIPs, json.RawMessage(tc.args))
		if err != nil {
			t.Fatalf("%s: %v", tc.args, err)
		}
		if got != tc.want {
			t.Errorf("%s: summary = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// 工具定义：query 不再全局 required；schema 携带 sort 说明（模型侧合同）。
func TestSearchIPsToolDefinitionSortParam(t *testing.T) {
	db := seedAgentGroundingDB(t)
	svc := searchIPsFixture(t, db, nil, []string{"game"})
	defs := svc.baseToolDefinitions()
	var searchIPs *llm.ToolDefinition
	for i := range defs {
		if defs[i].Name == ToolSearchIPs {
			searchIPs = &defs[i]
		}
	}
	if searchIPs == nil {
		t.Fatal("search_ips definition missing")
	}
	props := searchIPs.Parameters
	if _, has := props["required"]; has {
		t.Fatal("query must no longer be globally required (conditional contract lives in descriptions + backend validation)")
	}
	properties, ok := props["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties shape")
	}
	if _, has := properties["sort"]; !has {
		t.Fatal("sort parameter must be advertised for explicit browse")
	}
	sortProp, ok := properties["sort"].(map[string]interface{})
	if !ok || !strings.Contains(fmt.Sprint(sortProp["description"]), "newest") {
		t.Fatal("sort description must document newest/most_contents semantics")
	}
}
