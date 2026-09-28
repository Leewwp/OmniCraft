package service

import (
	"regexp"
	"strings"
	"unicode"
)

// FT-6 (#698)：M3 关思考态把推理直接当正文输出的窄域守卫。取证（trace
// f47a985e，2026-09-28）：deep_think OFF、无独立 think 行、正文无任何思考
// 标签，561 字符答案以 "The user is asking about…" 风格裸英文推理开头后接
// 中文答案，静默 SUCCESS；标签剥离防线（thinkSplitter）无标签即无物可剥。
// #544 裁定「不做通用裸文本启发式」——本守卫是有条件重开：条件收窄到误伤
// 面≈0 的三重合取（其一由调用方判定：仅 grounded_content 轮），config 一键
// 回滚（agent.answer_bare_reasoning_guard.enabled），命中记 trace 事件。
//
// 条件（函数内两重）：
//  1. 首个 CJK 字符之前存在至少一个完整英文句（字母开头、[.!?] 结尾）；
//  2. 首个 CJK 字符之后仍有 CJK 字符（其后存在中文正文——英文提问得英文
//     回答在此被拦截，不误伤）。
//
// 动作：剥离首个 CJK 字符之前的全部英文前缀（含剥离后残留的空白/开闭引
// 号），返回剥离的 rune 数（0 = 未命中，原样直通）。
var bareEnglishSentencePattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9 ,;'"\-()]*[.!?](\s|$)`)

func isCJKRune(r rune) bool { return unicode.Is(unicode.Han, r) }

// stripBareEnglishReasoningPrefix applies the two text-side conditions of the
// guard; the grounded-turn condition is the caller's (kind == grounded_content).
func stripBareEnglishReasoningPrefix(answer string) (string, int) {
	firstCJK := -1
	for i, r := range answer {
		if isCJKRune(r) {
			firstCJK = i
			break
		}
	}
	if firstCJK <= 0 {
		// 无 CJK（纯英文回答）或 CJK 已在开头（纯中文回答）：都不触发。
		return answer, 0
	}
	prefix := answer[:firstCJK]
	if !bareEnglishSentencePattern.MatchString(prefix) {
		return answer, 0
	}
	// 其后存在中文正文：首个 CJK 之后仍要有 CJK（单汉字后全英文视同英文
	// 回答，不触发）。
	rest := answer[firstCJK:]
	if !strings.ContainsFunc(rest[firstRuneSize(rest):], isCJKRune) {
		return answer, 0
	}
	stripped := strings.TrimLeft(rest, " \t\r\n'\"“”‘’()（）,，.。!！?？;；:：")
	if stripped == "" {
		return answer, 0
	}
	return stripped, len([]rune(answer)) - len([]rune(stripped))
}

// firstRuneSize returns the byte size of the first rune so the CJK-after check
// starts from the second CJK character onward.
func firstRuneSize(s string) int {
	for _, r := range s {
		return len(string(r))
	}
	return 0
}
