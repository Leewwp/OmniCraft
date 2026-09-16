package promptregistry

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/testutil"
)

// ---------------------------------------------------------------------------
// Byte-equivalence goldens: each frozen block below is a verbatim copy of
// the PRE-registry prompt assembly (fmt.Sprintf/builders from agent_service/
// agent_conversation/agent_stream/rag expander). If a slot template drifts
// from the original byte sequence, these tests fail. They are the
// "替换前后字节级等价" acceptance of T5 and must not be "fixed" by editing
// the expected strings to match a new template.
// ---------------------------------------------------------------------------

// oldServerOwnedSystemPrompt is the frozen original of
// AgentService.serverOwnedSystemPrompt (agent_service.go, pre-T5).
func oldServerOwnedSystemPrompt(surface string, contentParts []string, ipCategories []string) string {
	var parts []string
	parts = append(parts, surface)
	parts = append(parts, contentParts...)
	parts = append(parts,
		"when your answer relies on retrieved results, mark the sentence end with 1-based citation indexes like [1] or [2], where n is the position of the result in the search output you used; only mark results you actually used and keep the total number of distinct marks small",
		"for any request to find, search, recommend, compare or summarize site content, you must call the cited_search tool first and ground the answer only in its results; never recommend or describe site content from your own knowledge",
		"citation indexes are valid only within the turn that produced them: when the user asks for more, further, or additional recommendations (for example 「再推荐几个」), call the search tool again in that turn — never answer by reusing or restating results from earlier turns, because reused citations cannot be validated and the answer will be rejected",
		"never mention internal numeric content ids in your answer",
		"when the user's message contains a concrete title, quote, character name, or keyword that could exist on the site, always call the cited_search tool with it before replying, even if the intent seems ambiguous; for example, a message that is just a title like 「星轨下的制琴师」or 'A Quiet Ledger of Small Storms' is a search request: search that exact text first, then answer from the results, and only say you found nothing usable if the search comes back empty; only for pure greetings, thanks, farewells, or a message with no searchable text at all (for example garbled characters), reply briefly without any tool and without citation marks — one or two sentences in the user's language, either a greeting back or one clarifying question about what site content they need",
		"every search_content query must be fully self-contained: resolve all pronouns, ellipsis and context references into the concrete entities they point to (exact titles, author or character names, topics), so each query is understandable with zero prior conversation context; for example, when the user asks 「第二个的作者还有什么作品」 after earlier results, the query must be rewritten like 「《迟到的邮差》的作者的其他作品」 with the resolved title, never a bare reference such as 「第二个」 or 「它的作者」; a message that is just a bare title or quote is itself the self-contained query for its first search",
		"when one message combines several independent sub-questions, decompose it into multiple search_content calls — one call per sub-question, each with its own self-contained query — instead of merging them into a single vague query; the per-turn tool budget is sized for this",
	)
	if len(ipCategories) > 0 {
		parts = append(parts, fmt.Sprintf("when the user asks to find, recommend, or browse IPs (original settings/worlds), call the search_ips tool with a self-contained keyword query; when the user names a genre, pass category with one of these slugs only: %s; cite the IPs you used with the same [n] marks as content results", strings.Join(ipCategories, ", ")))
	}
	return "[OmniCraft Agent Context] " + strings.Join(parts, "; ")
}

func TestGoldenAgentSystemByteEquivalence(t *testing.T) {
	slot, ok := SlotByName("agent_system")
	if !ok {
		t.Fatal("agent_system slot missing")
	}
	cases := []struct {
		name         string
		surfaceParts []string
		ipCategories []string
	}{
		{"global", []string{"surface=global"}, nil},
		{"search", []string{"surface=search"}, nil},
		{"publish", []string{"surface=publish"}, nil},
		{"content minimal", []string{"surface=content"}, nil},
		{"content full", []string{"surface=content", "content_id=42", "title=迟到的邮差", "content_type=mod"}, nil},
		{"with ip categories", []string{"surface=global"}, []string{"game", "novel", "vtuber"}},
	}
	for _, tc := range cases {
		old := oldServerOwnedSystemPrompt(tc.surfaceParts[0], tc.surfaceParts[1:], tc.ipCategories)
		newPrompt := Render(slot.Builtin, map[string]string{
			"surface_context": strings.Join(tc.surfaceParts, "; "),
		})
		if len(tc.ipCategories) > 0 {
			newPrompt += "; " + fmt.Sprintf("when the user asks to find, recommend, or browse IPs (original settings/worlds), call the search_ips tool with a self-contained keyword query; when the user names a genre, pass category with one of these slugs only: %s; cite the IPs you used with the same [n] marks as content results", strings.Join(tc.ipCategories, ", "))
		}
		if newPrompt != old {
			t.Fatalf("case %q drifted:\n--- old ---\n%s\n--- new ---\n%s", tc.name, old, newPrompt)
		}
	}
}

func TestGoldenSidePromptByteEquivalence(t *testing.T) {
	cases := []struct {
		slot   string
		old    string
		values map[string]string
	}{
		{
			slot: "upload_assist_prompt",
			old: fmt.Sprintf(`You are a content tagging assistant for a fan content platform.
Given a file named "%s" of type "%s" with title "%s" and description "%s",
suggest appropriate tags, category, title improvements, and description.
Respond ONLY with valid JSON: {"suggested_tags":[],"suggested_category":"","suggested_title":"","suggested_description":""}`,
				"map.zip", "application/zip", "My Map", "A cool map"),
			values: map[string]string{"filename": "map.zip", "content_type": "application/zip", "title": "My Map", "description": "A cool map"},
		},
		{
			slot: "compliance_check_prompt",
			old: fmt.Sprintf(`Analyze the following content for compliance issues (copyright infringement, inappropriate content):
Title: %s
Description: %s
Type: %s

Respond ONLY with valid JSON: {"risk_level":"safe|warning|violation","reason":"","suggestions":[]}`,
				"T", "D", "mod"),
			values: map[string]string{"title": "T", "description": "D", "content_type": "mod"},
		},
		{
			slot: "usage_guide_prompt",
			old: fmt.Sprintf(`Generate a concise usage guide for this content:
Title: %s
Type: %s
Description: %s

Focus on: %s
Format as Markdown.`, "T", "guide", "D", "usage instructions and best practices"),
			values: map[string]string{"title": "T", "content_type": "guide", "description": "D", "guide_focus": "usage instructions and best practices"},
		},
		{
			slot: "content_moderation_prompt",
			old: fmt.Sprintf(`Moderate this content for policy violations:
Title: %s
Description: %s
Type: %s

Check for: copyright infringement, adult content, spam, hate speech.
Respond ONLY with JSON: {"risk_level":"safe|warning|violation","violations":[],"suggestions":[]}`,
				"T", "D", "article"),
			values: map[string]string{"title": "T", "description": "D", "content_type": "article"},
		},
		{
			slot: "conversation_title_prompt",
			old: fmt.Sprintf(
				"为下面的对话生成一个不超过 16 个字的简短中文标题，只输出标题本身，不要引号、序号或句号：\n%s",
				"帮我找一些类似的模组"),
			values: map[string]string{"first_user_message": "帮我找一些类似的模组"},
		},
		{
			slot: "query_expansion_prompt",
			old: fmt.Sprintf(
				"把下面的用户输入扩展为最多 %d 个用于站内内容检索的中文检索词，覆盖同义词、别名与相关表述，不要解释。只输出一个 JSON 字符串数组。\n输入：%s",
				5, "星穹铁道同人模组"),
			values: map[string]string{"max_terms": "5", "query": "星穹铁道同人模组"},
		},
	}
	for _, tc := range cases {
		slot, ok := SlotByName(tc.slot)
		if !ok {
			t.Fatalf("slot %s missing", tc.slot)
		}
		if got := Render(slot.Builtin, tc.values); got != tc.old {
			t.Fatalf("slot %s drifted:\n--- old ---\n%q\n--- new ---\n%q", tc.slot, tc.old, got)
		}
	}
}

// truncateRunes200 is the frozen followUpPrefixCap truncation the original
// followUpRequest applied to the answer prefix (followUpPrefixCap = 200).
func truncateRunes200(s string) string {
	runes := []rune(s)
	if len(runes) <= 200 {
		return s
	}
	return string(runes[:200])
}

func TestGoldenFollowUpsByteEquivalence(t *testing.T) {
	// Frozen copy of followUpRequest (agent_stream.go, pre-T5).
	oldFollowUp := func(question string, titles []string, answerPrefix string) string {
		var b strings.Builder
		b.WriteString("You suggest follow-up questions for a site-content assistant. ")
		b.WriteString("Based on the user's question, the retrieved result titles, and the beginning of the answer, propose 2-3 short follow-up questions the user might ask next about site content (works, IPs, usage). ")
		b.WriteString("Rules: one question per line, no numbering, no bullets; each question at most 20 characters; write in the same language as the user's question; output nothing else.\n")
		b.WriteString("User question: ")
		b.WriteString(strings.TrimSpace(question))
		b.WriteString("\nRetrieved titles: ")
		if len(titles) == 0 {
			b.WriteString("(none)")
		} else {
			b.WriteString(strings.Join(titles, " / "))
		}
		b.WriteString("\nAnswer beginning: ")
		b.WriteString(truncateRunes200(strings.TrimSpace(answerPrefix)))
		return b.String()
	}
	slot, _ := SlotByName("follow_ups_prompt")
	for _, tc := range []struct {
		question, prefix string
		titles           []string
	}{
		{"有哪些推荐？", "以下是三款……", []string{"A", "B"}},
		{"show me mods", "Here are", nil},
		{"long prefix", strings.Repeat("长", 500), nil},
	} {
		titles := "(none)"
		if len(tc.titles) > 0 {
			titles = strings.Join(tc.titles, " / ")
		}
		got := Render(slot.Builtin, map[string]string{
			"question":      strings.TrimSpace(tc.question),
			"titles":        titles,
			"answer_prefix": truncateRunes200(strings.TrimSpace(tc.prefix)),
		})
		if got != oldFollowUp(tc.question, tc.titles, tc.prefix) {
			t.Fatalf("follow_ups_prompt drifted:\n%q\nvs\n%q", got, oldFollowUp(tc.question, tc.titles, tc.prefix))
		}
	}
}

// ---------------------------------------------------------------------------
// Slot contract hygiene
// ---------------------------------------------------------------------------

func TestSlotsValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, slot := range Slots {
		if seen[slot.Name] {
			t.Fatalf("duplicate slot %s", slot.Name)
		}
		seen[slot.Name] = true
		if err := ValidateTemplate(slot, slot.Builtin); err != nil {
			t.Fatalf("builtin of %s violates its own contract: %v", slot.Name, err)
		}
	}
	if len(Slots) != 8 {
		t.Fatalf("slot count = %d, want 8 (full inventory)", len(Slots))
	}
}

func TestValidateTemplateRejectsDrift(t *testing.T) {
	slot, _ := SlotByName("conversation_title_prompt")
	if err := ValidateTemplate(slot, "no placeholders at all"); err == nil {
		t.Fatal("missing required placeholder must be rejected")
	}
	if err := ValidateTemplate(slot, "{{first_user_message}} {{extra}}"); err == nil {
		t.Fatal("unknown placeholder must be rejected")
	}
	if err := ValidateTemplate(slot, "{{first_user_message}}"); err != nil {
		t.Fatalf("exact contract must pass: %v", err)
	}
}

func TestRenderLeavesUnknownTokens(t *testing.T) {
	if got := Render("hello {{who}}", map[string]string{"x": "y"}); got != "hello {{who}}" {
		t.Fatalf("unknown token mutated: %q", got)
	}
	if got := Render("hello {{who}}", map[string]string{"who": "世界"}); got != "hello 世界" {
		t.Fatalf("render failed: %q", got)
	}
}

func TestTemplateTokensIgnoresJSONBraces(t *testing.T) {
	tokens := TemplateTokens(`{"a":1} {{title}} {"b":[1,2]}`)
	if len(tokens) != 1 || tokens[0] != "title" {
		t.Fatalf("tokens = %v, want [title]", tokens)
	}
}

// ---------------------------------------------------------------------------
// Resolver: cache, override, invalidation, fail-open
// ---------------------------------------------------------------------------

type fakeStore struct {
	mu      sync.Mutex
	byLabel map[string]*model.PromptRegistry
	err     error
}

func (f *fakeStore) GetByLabel(_ context.Context, name, label string) (*model.PromptRegistry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	row, ok := f.byLabel[name+"|"+label]
	if !ok {
		return nil, repository.ErrPromptNotFound
	}
	return row, nil
}

func (f *fakeStore) CreateVersion(_ context.Context, row *model.PromptRegistry) error {
	return nil
}

func (f *fakeStore) EnsureLabel(_ context.Context, name, label string, version int) error {
	return nil
}

func TestResolverBuiltinAndOverride(t *testing.T) {
	slot, _ := SlotByName("compliance_check_prompt")
	store := &fakeStore{byLabel: map[string]*model.PromptRegistry{}}
	r := NewPromptResolver(store)

	content, version := r.Resolve(context.Background(), slot)
	if content != slot.Builtin || version != 0 {
		t.Fatalf("empty registry must resolve builtin, got v%d", version)
	}

	override := &model.PromptRegistry{Name: slot.Name, Version: 3, Content: "rewritten {{title}} {{description}} {{content_type}}"}
	store.byLabel[slot.Name+"|"+ProductionLabel] = override
	r.Invalidate()
	content, version = r.Resolve(context.Background(), slot)
	if content != override.Content || version != 3 {
		t.Fatalf("production override not honored: v%d %q", version, content)
	}
	if got := r.RenderSlot(context.Background(), slot, map[string]string{"title": "T", "description": "D", "content_type": "mod"}); got != "rewritten T D mod" {
		t.Fatalf("RenderSlot = %q", got)
	}

	// Store failure degrades to builtin, never errors.
	store.err = errors.New("db down")
	r.Invalidate()
	content, version = r.Resolve(context.Background(), slot)
	if content != slot.Builtin || version != 0 {
		t.Fatal("registry failure must fall back to builtin")
	}
}

func TestResolverCachesLookups(t *testing.T) {
	slot, _ := SlotByName("agent_system")
	calls := 0
	store := &fakeStore{byLabel: map[string]*model.PromptRegistry{}}
	r := NewPromptResolver(&countingStore{inner: store, calls: &calls})
	for i := 0; i < 5; i++ {
		r.Resolve(context.Background(), slot)
	}
	if calls != 1 {
		t.Fatalf("store calls = %d, want 1 (TTL cache)", calls)
	}
	r.Invalidate()
	r.Resolve(context.Background(), slot)
	if calls != 2 {
		t.Fatalf("store calls after invalidate = %d, want 2", calls)
	}
}

type countingStore struct {
	inner *fakeStore
	calls *int
}

func (c *countingStore) GetByLabel(ctx context.Context, name, label string) (*model.PromptRegistry, error) {
	*c.calls++
	return c.inner.GetByLabel(ctx, name, label)
}
func (c *countingStore) CreateVersion(ctx context.Context, row *model.PromptRegistry) error {
	return c.inner.CreateVersion(ctx, row)
}
func (c *countingStore) EnsureLabel(ctx context.Context, name, label string, version int) error {
	return c.inner.EnsureLabel(ctx, name, label, version)
}

func TestNilResolverSafe(t *testing.T) {
	var r *PromptResolver
	slot, _ := SlotByName("agent_system")
	content, version := r.Resolve(context.Background(), slot)
	if content != slot.Builtin || version != 0 {
		t.Fatal("nil resolver must return builtin")
	}
	r.Invalidate() // must not panic
}

// ---------------------------------------------------------------------------
// SeedV1 against the real repository + migration
// ---------------------------------------------------------------------------

func TestSeedV1Idempotent(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, "../../../migrations/082_prompt_registry.sql")
	repo := repository.NewPromptRegistryRepository(db)
	ctx := context.Background()

	if err := SeedV1(ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := SeedV1(ctx, repo); err != nil {
		t.Fatalf("second seed must be a no-op: %v", err)
	}

	for _, slot := range Slots {
		row, err := repo.GetByLabel(ctx, slot.Name, ProductionLabel)
		if err != nil {
			t.Fatalf("slot %s has no production pointer: %v", slot.Name, err)
		}
		if row.Version != 1 || row.Content != slot.Builtin {
			t.Fatalf("slot %s v1 mismatch (len %d vs %d)", slot.Name, len(row.Content), len(slot.Builtin))
		}
	}

	// Admin moves production to a new version; re-seeding must not drag it
	// back to v1.
	if err := repo.CreateVersion(ctx, &model.PromptRegistry{Name: "agent_system", Version: 2, Content: "v2"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetLabel(ctx, "agent_system", ProductionLabel, 2); err != nil {
		t.Fatal(err)
	}
	if err := SeedV1(ctx, repo); err != nil {
		t.Fatal(err)
	}
	row, _ := repo.GetByLabel(ctx, "agent_system", ProductionLabel)
	if row.Version != 2 {
		t.Fatalf("re-seed moved admin pointer back to v1 (now v%d)", row.Version)
	}
}
