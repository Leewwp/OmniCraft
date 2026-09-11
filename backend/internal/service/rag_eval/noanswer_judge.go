package rageval

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Deterministic no-answer judge over the two contract strategies (§1.1).
// Judge-based scoring can be layered on top later; this judge is the
// executable floor every strategy run must pass, and it is fully
// reproducible offline.

// strict-not-found markers: the answer must explicitly state the corpus has
// no match. zh/en refusal phrasings the corpus prompts actually produce.
var strictNotFoundMarkers = []string{
	"找不到", "没有找到", "未找到", "暂无", "无法找到", "没有相关的", "没有相关内容",
	"库内没有", "没有找到相关", "查不到", "检索不到", "没有对应的内容", "没有对应的",
	// A-04 dev-run additions: honest phrasings qwen-plus actually produces
	// (previously judged as substitution hard-fails).
	"没有直接", "暂未收录", "没有收录", "检索结果中没有", "没有搜索到", "未收录", "未直接出现", "并未发现", "未发现",
	"not found", "no match", "does not appear", "does not exist", "doesn't appear", "couldn't find", "could not find", "no results", "no relevant",
}

// related-recommendation disclaimers: the answer must explicitly say there is
// no exact match before recommending similar works.
var relatedDisclaimerMarkers = []string{
	"没有完全对应", "没有完全匹配", "没有 exact 对应", "不完全对应", "没有严格对应", "找不到完全",
	"没有完全一致", "不完全一致", "no exact match", "not an exact match", "no exact",
	// A-04 dev-run additions (same lesson as the strict list).
	"没有直接描写", "没有直接对应", "没有直接涉及", "没有严格匹配", "没有确切的", "没有完全相符",
}

// NoAnswerJudgeInput carries everything the deterministic judge needs: the
// annotated strategy, the produced answer, its citations, the retrieval ids
// and the truth universe for recommendation existence checks.
type NoAnswerJudgeInput struct {
	Strategy      string
	Answer        string
	Citations     []AnswerEvalCitation
	RetrievedIDs  []int64
	ExpectedIDs   []int64 // expected tier (strictly matching works)
	AcceptableIDs []int64 // acceptable tier (similar-but-real works)
	MustNotClaim  []string
	// KnownContentIDs is the existence universe for recommendation-truth
	// checks: recommended ids must appear in it (corpus-visible set at run
	// time). Nil disables the existence check.
	KnownContentIDs map[int64]bool
}

// NoAnswerJudgeResult is the per-case judge outcome. HardFail marks the
// contract's hard-fail conditions (fabrication / passing a similar work off
// as the exact answer / recommending nonexistent content): the whole case
// fails regardless of other signals.
type NoAnswerJudgeResult struct {
	Strategy           string   `json:"strategy"`
	Pass               bool     `json:"pass"`
	HardFail           bool     `json:"hard_fail,omitempty"`
	Refused            bool     `json:"refused"`                       // strict: explicit not-found statement
	HonestDisclaimer   bool     `json:"honest_disclaimer"`             // related: explicit no-exact-match statement
	Fabricated         bool     `json:"fabricated,omitempty"`          // must_not_claim violation
	FakeExactClaim     bool     `json:"fake_exact_claim,omitempty"`    // similar work presented as the exact answer
	FakeRecommendation bool     `json:"fake_recommendation,omitempty"` // recommendation of nonexistent content
	ClaimedTitles      []string `json:"claimed_titles,omitempty"`      // violated must_not_claim entries
	Reason             string   `json:"reason,omitempty"`
}

// JudgeNoAnswer applies the dual-strategy rubric. An unknown strategy fails
// closed: the annotation contract only defines the two strategies, so a third
// value is a data error, not a pass.
func JudgeNoAnswer(input NoAnswerJudgeInput) NoAnswerJudgeResult {
	result := NoAnswerJudgeResult{Strategy: input.Strategy}
	switch input.Strategy {
	case NoAnswerStrictNotFound:
		result.Refused = containsAnyMarker(input.Answer, strictNotFoundMarkers) ||
			containsNegatedFind(input.Answer)
	case NoAnswerRelatedRecommendationOK:
		result.HonestDisclaimer = containsAnyMarker(input.Answer, relatedDisclaimerMarkers) ||
			containsAnyMarker(input.Answer, strictNotFoundMarkers) ||
			containsNegatedFind(input.Answer)
	default:
		result.Reason = fmt.Sprintf("unknown no_answer_strategy %q (want %q or %q)",
			input.Strategy, NoAnswerStrictNotFound, NoAnswerRelatedRecommendationOK)
		return result
	}

	// must_not_claim violations are hard fails under both strategies
	// (contract §1.1/§3: fabrication lives on the claim layer).
	for _, claim := range input.MustNotClaim {
		if strings.TrimSpace(claim) == "" {
			continue
		}
		if containsNormalizedText(input.Answer, claim) {
			result.Fabricated = true
			result.ClaimedTitles = append(result.ClaimedTitles, claim)
		}
	}

	recommended := recommendedIDs(input.Citations, input.RetrievedIDs, input.ExpectedIDs, input.AcceptableIDs)
	switch input.Strategy {
	case NoAnswerStrictNotFound:
		// The strict hard fail is substitution (相似文顶替为答案): citing a
		// similar work as the answer without ever stating the corpus has no
		// match. An explicit refusal followed by a framed recommendation is
		// not a substitution.
		if !result.Refused && len(input.Citations) > 0 {
			result.FakeExactClaim = true
		}
	case NoAnswerRelatedRecommendationOK:
		if !result.HonestDisclaimer && len(input.Citations) > 0 {
			// Recommending without declaring there is no exact match passes a
			// similar work off as the precise answer.
			result.FakeExactClaim = true
		}
		if input.KnownContentIDs != nil {
			for _, id := range recommended {
				if !input.KnownContentIDs[id] {
					result.FakeRecommendation = true
					break
				}
			}
		}
	}

	result.HardFail = result.Fabricated || result.FakeExactClaim || result.FakeRecommendation
	switch {
	case result.HardFail:
		result.Reason = hardFailReason(result)
	case input.Strategy == NoAnswerStrictNotFound && !result.Refused:
		result.Reason = "strict_not_found requires an explicit not-found statement"
	case input.Strategy == NoAnswerRelatedRecommendationOK && !result.HonestDisclaimer:
		result.Reason = "related_recommendation_allowed requires an explicit no-exact-match disclaimer"
	default:
		result.Reason = "pass"
	}
	result.Pass = !result.HardFail &&
		((input.Strategy == NoAnswerStrictNotFound && result.Refused) ||
			(input.Strategy == NoAnswerRelatedRecommendationOK && result.HonestDisclaimer))
	return result
}

func hardFailReason(result NoAnswerJudgeResult) string {
	parts := []string{}
	if result.Fabricated {
		parts = append(parts, fmt.Sprintf("must_not_claim violation: %v", result.ClaimedTitles))
	}
	if result.FakeExactClaim {
		parts = append(parts, "similar work presented as the exact answer")
	}
	if result.FakeRecommendation {
		parts = append(parts, "recommended content does not exist")
	}
	return strings.Join(parts, "; ")
}

// recommendedIDs derives the recommended content ids for the existence check:
// every cited id counts as a recommendation (a fabricated id never hides
// outside the expected/acceptable tiers); without citations the check falls
// back to retrieval hits inside the tiers (the diagnostic surface only).
func recommendedIDs(citations []AnswerEvalCitation, retrieved, expected, acceptable []int64) []int64 {
	tier := map[int64]bool{}
	for _, id := range expected {
		tier[id] = true
	}
	for _, id := range acceptable {
		tier[id] = true
	}
	var ids []int64
	if len(citations) > 0 {
		for _, c := range citations {
			ids = append(ids, c.ContentID)
		}
		return ids
	}
	seen := map[int64]bool{}
	for _, id := range retrieved {
		if tier[id] && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// containsNegatedFind matches English negated-existence sentences the fixed
// marker vocabulary cannot enumerate ("No cookbook recipes were found in the
// search results"): a negation token followed by find/match/result/locate
// within one sentence-clause span.
func containsNegatedFind(text string) bool {
	lower := strings.ToLower(text)
	negations := []string{"no ", "not ", "none ", "without ", "couldn't ", "cannot ", "can't ", "unable to "}
	findings := []string{"found", "match", "matches", "result", "results", "locate"}
	for _, neg := range negations {
		start := 0
		for {
			idx := strings.Index(lower[start:], neg)
			if idx < 0 {
				break
			}
			at := start + idx + len(neg)
			end := at + 80
			if end > len(lower) {
				end = len(lower)
			}
			clause := lower[at:end]
			if cut := strings.IndexAny(clause, ".。\n"); cut >= 0 {
				clause = clause[:cut]
			}
			for _, f := range findings {
				if strings.Contains(clause, f) {
					return true
				}
			}
			start = at
		}
	}
	return false
}

func containsAnyMarker(text string, markers []string) bool {
	normalized := normalizeClaimText(text)
	for _, marker := range markers {
		if marker == "" {
			continue
		}
		if strings.Contains(normalized, normalizeClaimText(marker)) || strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// containsNormalizedText checks a must_not_claim entry against the answer
// with claim-side normalisation: case folding, whitespace collapse and
// outer 《》 stripping, so 《星海遗响》 matches 星海遗响 inside prose.
func containsNormalizedText(answer, claim string) bool {
	answerNorm := normalizeClaimText(answer)
	claimNorm := normalizeClaimText(claim)
	if claimNorm == "" {
		return false
	}
	return strings.Contains(answerNorm, claimNorm)
}

func normalizeClaimText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "《")
	s = strings.TrimSuffix(s, "》")
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r):
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
		default:
			b.WriteRune(unicode.ToLower(r))
			lastSpace = false
		}
	}
	return b.String()
}
