package rageval

import (
	"fmt"
	"strings"
	"unicode"
)

// Deterministic assertion judge (freeze report §5 red line): the
// answer_rubric.deterministic_assertions entries are checked against the
// produced answer with punctuation normalisation and word-boundary matching
// so a short assertion can never pass as an accidental substring (#319
// lesson: half-width vs full-width punctuation and substring false hits).

// assertionMinRunes is the floor below which an assertion is treated as a
// data error instead of a matchable claim: single characters match almost
// everything and would make the judge meaningless.
const assertionMinRunes = 2

// latinWordBoundaryMinRunes: assertions shorter than this that contain no CJK
// runes are matched with word boundaries (non-alphanumeric on both sides) so
// short latin tokens cannot hit inside a longer word.
const latinWordBoundaryMinRunes = 12

// DeterministicAssertionResult is one assertion's outcome.
type DeterministicAssertionResult struct {
	Assertion string `json:"assertion"`
	Pass      bool   `json:"pass"`
	Invalid   bool   `json:"invalid,omitempty"` // too short / empty after normalisation
	Reason    string `json:"reason,omitempty"`
}

// DeterministicJudgeResult is the per-case outcome over all assertions.
type DeterministicJudgeResult struct {
	Total   int                            `json:"total"`
	Passed  int                            `json:"passed"`
	Invalid int                            `json:"invalid"`
	Pass    bool                           `json:"pass"`
	Results []DeterministicAssertionResult `json:"results,omitempty"`
}

// JudgeDeterministicAssertions checks every assertion against the answer.
// Empty answer fails every assertion. Invalid assertions (below the minimum
// length — the frozen set does carry a few single-rune claims like the
// currency unit 文) cannot be matched deterministically without substring
// false hits, so they are reported as invalid and EXCLUDED from the pass
// calculation; a case whose assertions are all invalid is unjudgeable and
// must be dropped from the layer denominator instead of scored as a fail.
func JudgeDeterministicAssertions(answer string, assertions []string) DeterministicJudgeResult {
	result := DeterministicJudgeResult{Total: len(assertions)}
	if strings.TrimSpace(answer) == "" {
		for _, a := range assertions {
			result.Results = append(result.Results, DeterministicAssertionResult{
				Assertion: a, Reason: "empty answer",
			})
		}
		result.Pass = len(assertions) == 0
		return result
	}
	answerNorm := NormalizeAssertionText(answer)
	for _, assertion := range assertions {
		r := DeterministicAssertionResult{Assertion: assertion}
		assertionNorm := NormalizeAssertionText(assertion)
		switch {
		case len([]rune(strings.TrimSpace(assertion))) < assertionMinRunes || assertionNorm == "":
			r.Invalid = true
			r.Reason = fmt.Sprintf("assertion below minimum length (%d runes)", assertionMinRunes)
			result.Invalid++
		case assertionContainsCJK(assertion):
			r.Pass = strings.Contains(answerNorm, assertionNorm)
		case len([]rune(assertionNorm)) < latinWordBoundaryMinRunes:
			r.Pass = containsWithBoundaries(answerNorm, assertionNorm)
		default:
			r.Pass = strings.Contains(answerNorm, assertionNorm)
		}
		if !r.Pass && r.Reason == "" {
			r.Reason = "not found in answer"
		}
		if r.Pass {
			result.Passed++
		}
		result.Results = append(result.Results, r)
	}
	judgeable := result.Total - result.Invalid
	result.Pass = result.Total == 0 || (judgeable > 0 && result.Passed == judgeable)
	return result
}

// NormalizeAssertionText folds an assertion or answer to the judge's
// comparison space: unicode case folding, whitespace collapse and
// full-width→half-width punctuation mapping (：，。！？；（） etc.), so
// "半角: 冒号" and "全角：冒号" compare equal.
func NormalizeAssertionText(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		default:
			b.WriteRune(foldWidth(r))
			lastSpace = false
		}
	}
	return b.String()
}

// foldWidth maps the full-width/half-width pairs that matter for assertion
// matching. Letters/digits already went through ToLower; only punctuation
// needs explicit mapping here.
func foldWidth(r rune) rune {
	switch r {
	case '：':
		return ':'
	case '，':
		return ','
	case '。':
		return '.'
	case '！':
		return '!'
	case '？':
		return '?'
	case '；':
		return ';'
	case '（':
		return '('
	case '）':
		return ')'
	case '“', '”':
		return '"'
	case '‘', '’':
		return '\''
	case '、':
		return ','
	case '—', '–':
		return '-'
	}
	// full-width ASCII block (Ａ-Ｚ ａ-ｚ ０-９ and ASCII punctuation) folds to
	// its half-width counterpart (U+FF01..U+FF5E maps onto U+0021..U+007E).
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFF01 + '!'
	}
	return r
}

func assertionContainsCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// containsWithBoundaries matches needle in haystack requiring a
// non-alphanumeric boundary on both sides of every hit, so the short token
// "ink" matches "inkblot test" but not "thinking".
func containsWithBoundaries(haystack, needle string) bool {
	if needle == "" {
		return false
	}
	isWord := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r)
	}
	start := 0
	for {
		idx := strings.Index(haystack[start:], needle)
		if idx < 0 {
			return false
		}
		at := start + idx
		end := at + len(needle)
		beforeOK := at == 0 || !isWord(rune(haystack[at-1]))
		afterOK := end >= len(haystack) || !isWord(rune(haystack[end]))
		if beforeOK && afterOK {
			return true
		}
		start = at + 1
	}
}
