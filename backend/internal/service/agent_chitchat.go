package service

import (
	"strings"
	"unicode"
)

// SP-15 A1 rule-layer chitchat shortcut: an exact-match greeting or farewell
// is answered by a server-owned template before any Provider call. The reply
// is conversational by construction (zero tools, zero citations), the turn
// still consumes its reserved quota, and both messages still persist.

// chitchatMaxRunes bounds the exact-match table: longer messages can never
// shortcut, they always reach the model layer. 9 admits the longest factory
// keyword "thank you" (9 runes including the collapsed space).
const chitchatMaxRunes = 9

// foldChitchatRune maps full-width ASCII variants and the ideographic space to
// their half-width forms, so "ｈｉ" and "hi" (or "你好　" with a full-width
// trailing space) normalize identically.
func foldChitchatRune(r rune) rune {
	switch {
	case r == 0x3000:
		return ' '
	case r >= 0xFF01 && r <= 0xFF5E:
		return r - 0xFEE0
	default:
		return r
	}
}

// normalizeChitchatText applies the shortcut's exact-match normalization:
// trim, lowercase, fold full-width forms, collapse whitespace runs. Matching
// stays whole-message equality — "你好，帮我找点东西" never matches "你好".
func normalizeChitchatText(s string) string {
	folded := strings.Map(foldChitchatRune, strings.ToLower(strings.TrimSpace(s)))
	return strings.Join(strings.Fields(folded), " ")
}

// chitchatIntentByPattern routes each factory keyword to one template intent.
// The pattern table itself lives in config (agent.chitchat_patterns); a
// configured pattern without a code-side intent falls back to the greeting
// template, so operators can extend the table without code changes.
var chitchatIntentByPattern = map[string]string{
	"你好":    "greet",
	"您好":    "greet",
	"嗨":     "greet",
	"哈喽":    "greet",
	"hello":  "greet",
	"hi":     "greet",
	"hey":    "greet",
	"在吗":    "greet",
	"谢谢":    "thanks",
	"多谢":    "thanks",
	"感谢":    "thanks",
	"thanks": "thanks",
	"thank you": "thanks",
	"再见":        "bye",
	"拜拜":        "bye",
	"晚安":        "bye",
	"早上好":       "bye",
	"中午好":       "bye",
	"下午好":       "bye",
	"晚上好":       "bye",
}

// chitchatTemplates carries one zh/en template per intent. Copy stays neutral:
// acknowledge, state what the agent can do, never pretend the message carried
// a content request.
var chitchatTemplates = map[string]struct{ zh, en string }{
	"greet": {
		zh: "你好，我是 OmniCraft Agent，可以帮你找站内的作品、IP 或使用说明。",
		en: "Hi, I'm the OmniCraft Agent — I can help you find site works, IPs or usage guides.",
	},
	"thanks": {
		zh: "不客气！还想找什么作品或 IP，随时告诉我。",
		en: "You're welcome! Tell me anytime if you're looking for more works or IPs.",
	},
	"bye": {
		zh: "再见！之后想找站内内容随时回来。",
		en: "See you! Come back anytime you need to find site content.",
	},
}

// containsCJK reports whether the raw message uses CJK script, selecting the
// zh template; anything else gets the en template.
func containsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) {
			return true
		}
	}
	return false
}

// chitchatShortcutReply reports the server-owned template for an exact-match
// chitchat message, and whether the shortcut applies at all.
func (s *AgentService) chitchatShortcutReply(message string) (string, bool) {
	if s.cfg == nil || !s.cfg.Agent.ChitchatShortcutEnabled || len(s.cfg.Agent.ChitchatPatterns) == 0 {
		return "", false
	}
	normalized := normalizeChitchatText(message)
	if normalized == "" || len([]rune(normalized)) > chitchatMaxRunes {
		return "", false
	}
	for _, pattern := range s.cfg.Agent.ChitchatPatterns {
		if normalizeChitchatText(pattern) != normalized {
			continue
		}
		intent, ok := chitchatIntentByPattern[normalized]
		if !ok {
			intent = "greet"
		}
		template := chitchatTemplates[intent]
		if containsCJK(message) {
			return template.zh, true
		}
		return template.en, true
	}
	return "", false
}
