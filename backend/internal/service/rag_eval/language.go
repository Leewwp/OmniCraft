package rageval

import (
	"strings"
	"unicode"
)

// ClassifyQueryLanguage implements the golden-set language rule (rag design
// §6, confirmed 2026-08-11): strip punctuation, then compute the Chinese
// character share of the remaining text.
//
//	share > 50%            -> "zh"
//	share in [10%, 50%]    -> "mixed"
//	share < 10%            -> "en"   (pure-ASCII queries land here with share 0)
//
// The bands are mutually exclusive, so every query gets exactly one label.
func ClassifyQueryLanguage(query string) string {
	total, chinese := 0, 0
	for _, r := range query {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) || unicode.IsSpace(r) {
			continue
		}
		total++
		if isCJK(r) {
			chinese++
		}
	}
	if total == 0 {
		return "en"
	}
	share := float64(chinese) / float64(total)
	switch {
	case share > 0.5:
		return "zh"
	case share >= 0.1:
		return "mixed"
	default:
		return "en"
	}
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF // CJK Unified Ideographs
}

// wordTokens splits text into lowercased word tokens (letters and digits
// runs), dropping punctuation and whitespace. CJK runs are letters, so a
// Chinese title stays as contiguous word tokens, matching PostgreSQL's
// simple text-search tokenizer that the production search path uses.
func wordTokens(text string) []string {
	var tokens []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			tokens = append(tokens, strings.ToLower(string(current)))
			current = current[:0]
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}
