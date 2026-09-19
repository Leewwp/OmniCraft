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

// Slots is the full, ordered inventory of prompt sites (user decision
// 2026-09-16: ALL slots enter the registry, not only high-traffic ones).
// v1 of every slot is byte-identical to the pre-registry hardcoded prompt;
// golden tests in promptregistry_test.go pin that equivalence.
var (
	SlotAgentSystem = PromptSlot{
		Name:        "agent_system",
		Description: "主 Agent 系统提示词（surface 上下文前缀 + 检索/引用/会话车道指令；IP 类目子句由 config 追加，不入模板）",
		Builtin:     agentSystemBuiltin(),
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
		Description: "内容使用指南生成（Markdown 输出）",
		Builtin: `Generate a concise usage guide for this content:
Title: {{title}}
Type: {{content_type}}
Description: {{description}}

Focus on: {{guide_focus}}
Format as Markdown.`,
		RequiredPlaceholders: []string{"content_type", "description", "guide_focus", "title"},
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
