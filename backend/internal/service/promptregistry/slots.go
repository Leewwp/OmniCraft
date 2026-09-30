// Package promptregistry is the versioned prompt registry (SP-21 T5, map
// #549): every prompt template the agent stack renders lives here as a
// PromptSlot with a builtin template and a required-placeholder contract.
// The database (prompt_registry + prompt_labels) layers immutable versions
// and production/staging pointers on top; resolution is builtin-first with
// the DB production version overriding, so the agent keeps working with the
// compiled-in prompt when the registry is empty or unreachable.
package promptregistry

import (
	"fmt"
	"sort"
	"strings"
)

// PromptSlot describes one prompt site: its registry name, what it is for,
// the builtin (v1) template, and the placeholders a saved template must
// keep providing (validated at save time, polyu slot-enum lesson).
type PromptSlot struct {
	Name                 string
	Description          string
	Builtin              string
	RequiredPlaceholders []string
	// Fallback overrides Builtin as the no-registry / unreachable fallback
	// template (#754 D). Empty = Builtin. The agent_system slot keeps its
	// v1 corpus as Builtin (v2+ upgrades must stay prefix-extensions of it
	// and SeedV1 keeps inserting the historical v1), while a registry-less
	// runtime still gets corrected tool instructions.
	Fallback string
}

// fallbackTemplate returns the template served when the registry is absent
// or unreachable.
func (s PromptSlot) fallbackTemplate() string {
	if s.Fallback != "" {
		return s.Fallback
	}
	return s.Builtin
}

// Render substitutes {{key}} tokens with values. Unknown tokens are left
// untouched so an obvious {{typo}} surfaces in the digest instead of
// silently deleting content; required keys missing from values render as
// empty strings.
func Render(template string, values map[string]string) string {
	if len(values) == 0 || template == "" {
		return template
	}
	pairs := make([]string, 0, len(values)*2)
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pairs = append(pairs, "{{"+k+"}}", values[k])
	}
	return strings.NewReplacer(pairs...).Replace(template)
}

// TemplateTokens returns every {{token}} appearing in a template, sorted.
// Save-time validation requires this set to equal the slot's required set:
// a missing required placeholder breaks rendering at runtime, an unknown
// token means the template references data no caller provides.
func TemplateTokens(template string) []string {
	seen := map[string]bool{}
	var tokens []string
	for i := 0; i+len("{{}}") <= len(template); {
		start := strings.Index(template[i:], "{{")
		if start < 0 {
			break
		}
		start += i
		end := strings.Index(template[start:], "}}")
		if end < 0 {
			break
		}
		token := template[start+2 : start+end]
		if token != "" && !seen[token] && !strings.ContainsAny(token, "{}") {
			seen[token] = true
			tokens = append(tokens, token)
		}
		i = start + end + 2
	}
	sort.Strings(tokens)
	return tokens
}

// ValidateTemplate checks that a candidate template's token set equals the
// slot's required placeholders exactly.
func ValidateTemplate(slot PromptSlot, template string) error {
	got := TemplateTokens(template)
	want := append([]string(nil), slot.RequiredPlaceholders...)
	sort.Strings(want)
	if strings.Join(got, "\x00") == strings.Join(want, "\x00") {
		return nil
	}
	return fmt.Errorf("template placeholders %v do not match required %v", got, want)
}

// SlotByName looks a slot up; unknown names are rejected before any DB I/O.
func SlotByName(name string) (PromptSlot, bool) {
	for _, s := range Slots {
		if s.Name == name {
			return s, true
		}
	}
	return PromptSlot{}, false
}

// agentSystemInstructions is the ordered instruction corpus of the main
// agent system prompt (moved verbatim from agent_service.go so the registry
// is the single authority; per-instruction rationale lives in git history).
var agentSystemInstructions = []string{
	// A-06 inline citation anchoring.
	"when your answer relies on retrieved results, mark the sentence end with 1-based citation indexes like [1] or [2], where n is the position of the result in the search output you used; only mark results you actually used and keep the total number of distinct marks small",
	// Must-search before answering from memory.
	"for any request to find, search, recommend, compare or summarize site content, you must call the cited_search tool first and ground the answer only in its results; never recommend or describe site content from your own knowledge",
	// #535 follow-up turns must re-search, never reuse citations.
	"citation indexes are valid only within the turn that produced them: when the user asks for more, further, or additional recommendations (for example 「再推荐几个」), call the search tool again in that turn — never answer by reusing or restating results from earlier turns, because reused citations cannot be validated and the answer will be rejected",
	"never mention internal numeric content ids in your answer",
	// SP-15 A2 conversational lane.
	"when the user's message contains a concrete title, quote, character name, or keyword that could exist on the site, always call the cited_search tool with it before replying, even if the intent seems ambiguous; for example, a message that is just a title like 「星轨下的制琴师」or 'A Quiet Ledger of Small Storms' is a search request: search that exact text first, then answer from the results, and only say you found nothing usable if the search comes back empty; only for pure greetings, thanks, farewells, or a message with no searchable text at all (for example garbled characters), reply briefly without any tool and without citation marks — one or two sentences in the user's language, either a greeting back or one clarifying question about what site content they need",
	// SP-15 D1 self-contained rewrite.
	"every search_content query must be fully self-contained: resolve all pronouns, ellipsis and context references into the concrete entities they point to (exact titles, author or character names, topics), so each query is understandable with zero prior conversation context; for example, when the user asks 「第二个的作者还有什么作品」 after earlier results, the query must be rewritten like 「《迟到的邮差》的作者的其他作品」 with the resolved title, never a bare reference such as 「第二个」 or 「它的作者」; a message that is just a bare title or quote is itself the self-contained query for its first search",
	// SP-15 D2 sub-question decomposition.
	"when one message combines several independent sub-questions, decompose it into multiple search_content calls — one call per sub-question, each with its own self-contained query — instead of merging them into a single vague query; the per-turn tool budget is sized for this",
}

// agentSystemBuiltin assembles the main system prompt template. The config
// conditional IP clause (SP-19 G2-1) is appended by code after rendering —
// it depends on config, not on prompt management.
func agentSystemBuiltin() string {
	return "[OmniCraft Agent Context] {{surface_context}}; " + strings.Join(agentSystemInstructions, "; ")
}

// agentSystemV2Extra is the single instruction v2 appends to the v1 corpus
// (#610): image requests must actually run the tool. Live evidence (session
// #1083) showed the model replying to 「再画一张：竖版构图」 with text only,
// the strict citation gate then wiped the body (tools:0 answer_kind:
// no_evidence), and later turns hallucinated the never-generated image into
// the series count. The exemption clause keeps ideation requests (「帮我构思
// 画面」) on the normal advice path.
const agentSystemV2Extra = "when the user asks to generate, create, draw, paint, or modify an image (for example 「画一张水彩风格的灯塔」 or 「再画一张：竖版构图」), you must call the generate_image tool in that same turn with a self-contained image prompt instead of replying with a text description alone; never claim, describe, or count images that were not actually generated by the tool in this conversation; requests that only ask for ideas, advice, or brainstorming about a picture without wanting an actual image (for example 「帮我构思画面」) are exempt and answered as normal advice"

// agentSystemV2 is the #610 upgrade of agent_system: the full v1 corpus
// byte-identical plus the image-tool guidance above. v1 stays untouched so
// the registry diff and rollback target keep working.
func agentSystemV2() string {
	return agentSystemBuiltin() + "; " + agentSystemV2Extra
}

// agentSystemV3Citation replaces the v1 per-search positional citation
// instruction (FT-5 #697). Live evidence (2026-09-28 DB forensics) showed
// the structural misalignment: the model numbered [n] by the local position
// within one search call's output while the emitted citation pool
// accumulates across every call of the turn — in a two-search_ips turn the
// model's [2] (second call, local #1) landed on the pool's position 2 (first
// call, second result), so the badge jumped to the wrong card. v3 points the
// model at the turn-global "cite" number the server stamps next to every
// result in the tool output.
const agentSystemV3Citation = "when your answer relies on retrieved results, mark the sentence end with 1-based citation indexes like [1] or [2], where n is the global cite number printed next to each result in this turn's tool outputs (cite numbers accumulate across all search calls of the same turn; a result shown without a cite number cannot be cited); only cite results you actually used and keep the total number of distinct marks small"

// agentSystemV3 is the FT-5 upgrade of agent_system: the v2 corpus with the
// citation instruction swapped for the turn-global numbering clause (and
// nothing else — the no-reasoning clause belongs to v4, retrieval-first to a
// possible v5). v1/v2 stay byte-identical so the registry diff and rollback
// chain keep working.
func agentSystemV3() string {
	corpus := append([]string(nil), agentSystemInstructions...)
	corpus[0] = agentSystemV3Citation // index 0 = the A-06 citation clause above
	return "[OmniCraft Agent Context] {{surface_context}}; " + strings.Join(corpus, "; ") + "; " + agentSystemV2Extra
}

// agentSystemV4NoReasoning is the single instruction v4 appends to the v3
// corpus (FT-6 #698). Live evidence (trace f47a985e, 2026-09-28): with
// thinking disabled M3 sometimes emits its reasoning as the answer body
// itself — a bare English "The user is asking about…" run followed by the
// Chinese answer, no think tags for the splitter to strip. v4 tells the
// model the body must be the final answer only; the narrow server-side
// guard (agent_answer_guard.go) is the belt-and-braces backstop.
const agentSystemV4NoReasoning = "never include your reasoning, thinking process, or chain of thought in the answer body; compose the final answer directly in the user's language — the body must contain only the answer itself, never the steps you took to reach it"

// agentSystemV4 is the FT-6 upgrade of agent_system: the v3 corpus plus the
// no-reasoning-in-body clause and nothing else. v1–v3 stay byte-identical so
// the registry diff and rollback chain keep working.
func agentSystemV4() string {
	return agentSystemV3() + "; " + agentSystemV4NoReasoning
}

// agentSystemV5RetrievalFirst is the single instruction v5 appends to the v4
// corpus (FT-7 #629, diagnosis-driven). Demo-site trace forensics (three
// 2026-09-20 turns, e.g. 1cd24f5f): idea/planning questions about site
// content or IPs — 「帮我想一个系列」「怎么融合这两个主题」 — were answered
// from imagination with zero tool calls, so the strict citation gate cleared
// every answer to no_evidence. The existing must-search clause lists
// find/search/recommend/compare/summarize but not ideation; v5 closes that
// gap. Pure general brainstorming unrelated to site content stays exempt
// (short conversational lane), as does image ideation (v2 exemption).
const agentSystemV5RetrievalFirst = "when the user asks for ideas, suggestions, plans, series, or creative concepts about site content or IPs (for example 「帮我想一个系列」 or 「怎么融合这两个主题」), you must call the search tool first with the relevant topics and ground the suggestions in what actually exists on the site; never answer such requests purely from imagination, because an ungrounded long answer is cleared as no evidence"

// agentSystemV5 is the FT-7 upgrade of agent_system: the v4 corpus plus the
// retrieval-first clause and nothing else. v1–v4 stay byte-identical so the
// registry diff and rollback chain keep working.
func agentSystemV5() string {
	return agentSystemV4() + "; " + agentSystemV5RetrievalFirst
}

// #754 D：v6 勘正两处幽灵工具名 cited_search（v1 corpus 的 must-search 条款
// 与 SP-15 A2 关键词条款），并追加 search_ips 显式浏览指导（sort=newest /
// most_contents + 空 query；热门诉求按内容量近似披露依据，不伪造榜单）。
// v1–v5 逐字节不动；v6 以 corpus 索引替换方式组装，保持 v3 以来的拼装形态。
const agentSystemV6MustSearch = "for any request to find, search, recommend, compare or summarize site content, you must call search_content first — search_ips for IP (original settings/worlds) requests — and ground the answer only in their results; never recommend or describe site content or IPs from your own knowledge"

const agentSystemV6KeywordFirst = "when the user's message contains a concrete title, quote, character name, or keyword that could exist on the site, always call search_content (or search_ips for IP requests) with it before replying, even if the intent seems ambiguous; for example, a message that is just a title like 「星轨下的制琴师」or 'A Quiet Ledger of Small Storms' is a search request: search that exact text first, then answer from the results, and only say you found nothing usable if the search comes back empty; only for pure greetings, thanks, farewells, or a message with no searchable text at all (for example garbled characters), reply briefly without any tool and without citation marks — one or two sentences in the user's language, either a greeting back or one clarifying question about what site content they need"

const agentSystemV6Browse = "for browse or listing requests without a specific keyword (for example 「最近热门的 ip」 or 「有哪些新的 IP」), call search_ips with sort=newest or sort=most_contents and an empty query, optionally with a category filter, instead of guessing a keyword; when the user asks for hot, trending, or recent items, describe the actual ordering basis honestly — most_contents orders by published-content count as an activity approximation, newest by creation time; there is no real-time popularity score or time-window ranking, so never fabricate entries, rankings, or recency"

// agentSystemV6 assembles the #754 D upgrade of agent_system: the v5 corpus
// with the two cited_search clauses swapped for corrected tool names, plus
// the explicit-browse guidance appended. v1–v5 stay byte-identical so the
// registry diff and rollback chain keep working.
func agentSystemV6() string {
	corpus := append([]string(nil), agentSystemInstructions...)
	corpus[0] = agentSystemV3Citation   // index 0 = v3 citation swap (kept)
	corpus[1] = agentSystemV6MustSearch // index 1 = must-search ghost fix
	corpus[4] = agentSystemV6KeywordFirst
	return "[OmniCraft Agent Context] {{surface_context}}; " + strings.Join(corpus, "; ") + "; " + agentSystemV2Extra + "; " + agentSystemV4NoReasoning + "; " + agentSystemV5RetrievalFirst + "; " + agentSystemV6Browse
}

// usageGuideV2LanguageClause is the single instruction v2 appends to the v1
// usage-guide corpus (#723): the guide's output language follows the
// requester's locale. Before v2 the language was implicit (English prompt,
// unspecified output); the panel surfaced whatever the model chose.
const usageGuideV2LanguageClause = "Write the entire guide in {{language}}."

// usageGuideV2 is the #723 upgrade of usage_guide_prompt: the v1 corpus plus
// the output-language clause and nothing else. v1 stays byte-identical so the
// registry diff and rollback chain keep working.
func usageGuideV2() string {
	return `Generate a concise usage guide for this content:
Title: {{title}}
Type: {{content_type}}
Description: {{description}}

Focus on: {{guide_focus}}
` + usageGuideV2LanguageClause + `
Format as Markdown.`
}

// Slots is the full, ordered inventory of prompt sites (user decision
// 2026-09-16: ALL slots enter the registry, not only high-traffic ones).
// v1 of every slot is byte-identical to the pre-registry hardcoded prompt;
// golden tests in promptregistry_test.go pin that equivalence.
var (
	SlotAgentSystem = PromptSlot{
		Name:        "agent_system",
		Description: "主 Agent 系统提示词（surface 上下文前缀 + 检索/引用/会话车道指令；IP 类目子句由 config 追加，不入模板）",
		Builtin:     agentSystemBuiltin(),
		// #754 D：无 registry / 读取失败时的 fallback 服务 v6（勘正工具名
		// + 浏览指导）；Builtin 仍为 v1 corpus（v2+ 升级的前缀不变量与
		// SeedV1 的历史 v1 种子均不动）。
		Fallback: agentSystemV6(),
		RequiredPlaceholders: []string{
			"surface_context",
		},
	}
	SlotUploadAssist = PromptSlot{
		Name:        "upload_assist_prompt",
		Description: "上传元数据助手（标签/类目/标题/描述建议，JSON 契约）",
		Builtin: `You are a content tagging assistant for a fan content platform.
Given a file named "{{filename}}" of type "{{content_type}}" with title "{{title}}" and description "{{description}}",
suggest appropriate tags, category, title improvements, and description.
Respond ONLY with valid JSON: {"suggested_tags":[],"suggested_category":"","suggested_title":"","suggested_description":""}`,
		RequiredPlaceholders: []string{"content_type", "description", "filename", "title"},
	}
	SlotComplianceCheck = PromptSlot{
		Name:        "compliance_check_prompt",
		Description: "版权/合规检查分析提示词（JSON 契约）",
		Builtin: `Analyze the following content for compliance issues (copyright infringement, inappropriate content):
Title: {{title}}
Description: {{description}}
Type: {{content_type}}

Respond ONLY with valid JSON: {"risk_level":"safe|warning|violation","reason":"","suggestions":[]}`,
		RequiredPlaceholders: []string{"content_type", "description", "title"},
	}
	SlotUsageGuide = PromptSlot{
		Name:        "usage_guide_prompt",
		Description: "内容使用指南生成（Markdown 输出；v2 起输出语言跟随 locale）",
		// #723：Builtin 即 v2 内容（存量库 v1 行不可变，经 RegistryUpgrades
		// 升版；新库 SeedV1 直接落该内容）——保持「builtin 满足自身契约」
		// 不变量（TestSlotsValidAndUnique）。
		Builtin:              usageGuideV2(),
		RequiredPlaceholders: []string{"content_type", "description", "guide_focus", "language", "title"},
	}
	SlotContentModeration = PromptSlot{
		Name:        "content_moderation_prompt",
		Description: "内容审核综合分析（JSON 契约）",
		Builtin: `Moderate this content for policy violations:
Title: {{title}}
Description: {{description}}
Type: {{content_type}}

Check for: copyright infringement, adult content, spam, hate speech.
Respond ONLY with JSON: {"risk_level":"safe|warning|violation","violations":[],"suggestions":[]}`,
		RequiredPlaceholders: []string{"content_type", "description", "title"},
	}
	SlotConversationTitle = PromptSlot{
		Name:                 "conversation_title_prompt",
		Description:          "会话自动标题生成（16 字中文短标题）",
		Builtin:              "为下面的对话生成一个不超过 16 个字的简短中文标题，只输出标题本身，不要引号、序号或句号：\n{{first_user_message}}",
		RequiredPlaceholders: []string{"first_user_message"},
	}
	SlotQueryExpansion = PromptSlot{
		Name:                 "query_expansion_prompt",
		Description:          "检索查询扩展（中文检索词 JSON 数组；system 行固定在代码，只模板化 user 提示词）",
		Builtin:              "把下面的用户输入扩展为最多 {{max_terms}} 个用于站内内容检索的中文检索词，覆盖同义词、别名与相关表述，不要解释。只输出一个 JSON 字符串数组。\n输入：{{query}}",
		RequiredPlaceholders: []string{"max_terms", "query"},
	}
	SlotFollowUps = PromptSlot{
		Name:        "follow_ups_prompt",
		Description: "追问建议生成（2-3 条同语言短问题，行输出）",
		Builtin: `You suggest follow-up questions for a site-content assistant. Based on the user's question, the retrieved result titles, and the beginning of the answer, propose 2-3 short follow-up questions the user might ask next about site content (works, IPs, usage). Rules: one question per line, no numbering, no bullets; each question at most 20 characters; write in the same language as the user's question; output nothing else.
User question: {{question}}
Retrieved titles: {{titles}}
Answer beginning: {{answer_prefix}}`,
		RequiredPlaceholders: []string{"answer_prefix", "question", "titles"},
	}
	// SP-24 R4 contextual retrieval: ingestion-side chunk situation prefix
	// (Anthropic-style). The rendered prompt is the user message; the fixed
	// system line lives in the annotator (expander pattern).
	SlotContextualAnnotation = PromptSlot{
		Name:        "contextual_annotation_prompt",
		Description: "chunk 上下文定位前缀生成（contextual retrieval 摄入侧，50-100 token 一句话，同语言输出）",
		Builtin: `为改善检索，请为下面这个文档片段写一句简短的上下文定位说明，帮助读者在不阅读全文的情况下理解该片段在整篇文档中的位置与主题。
文档标题：{{title}}
文档内容（可能截断）：{{document}}
待定位片段：{{chunk}}
要求：一句话，不超过 {{max_tokens}} 个 token；说明片段相对全文的位置（如所属章节/情节阶段/讨论主题）与它讲什么；与片段相同语言；只输出这句话本身，不要引号、编号、「上下文：」之类的标签或任何解释。`,
		RequiredPlaceholders: []string{"chunk", "document", "max_tokens", "title"},
	}
)

// Slots iterates the full inventory (seeding, admin listings).
var Slots = []PromptSlot{
	SlotAgentSystem,
	SlotUploadAssist,
	SlotComplianceCheck,
	SlotUsageGuide,
	SlotContentModeration,
	SlotConversationTitle,
	SlotQueryExpansion,
	SlotFollowUps,
	SlotContextualAnnotation,
}
