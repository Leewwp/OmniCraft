package rageval

import "testing"

func citation(id int64) AnswerEvalCitation { return AnswerEvalCitation{ContentID: id} }

// H6 strict_not_found: explicit not-found statement passes with zero
// fabrication.
func TestJudgeNoAnswerStrictPass(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy: NoAnswerStrictNotFound,
		Answer:   "库内没有找到《星海遗响》这篇作品，无法提供相关内容。",
	})
	if !result.Pass || result.HardFail || !result.Refused {
		t.Fatalf("strict pass = %+v", result)
	}
}

func TestJudgeNoAnswerStrictRefusalInEnglish(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy: NoAnswerStrictNotFound,
		Answer:   "No match found in the library for that title.",
	})
	if !result.Pass || !result.Refused {
		t.Fatalf("strict en pass = %+v", result)
	}
}

// strict failure: no not-found statement at all.
func TestJudgeNoAnswerStrictMissingRefusal(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy: NoAnswerStrictNotFound,
		Answer:   "这篇作品讲述了一个宏大的星际故事。",
	})
	if result.Pass || result.HardFail || result.Refused {
		t.Fatalf("strict missing refusal = %+v, want soft fail", result)
	}
}

// strict hard fail: citing a similar work as the answer without ever saying
// the corpus has no match (相似文顶替为答案).
func TestJudgeNoAnswerStrictSubstitutionHardFail(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:  NoAnswerStrictNotFound,
		Answer:    "《星海回响》正是你要找的作品，讲的是……",
		Citations: []AnswerEvalCitation{citation(5)},
	})
	if !result.HardFail || !result.FakeExactClaim || result.Pass {
		t.Fatalf("strict substitution = %+v, want hard fail", result)
	}
}

// strict + explicit refusal + framed citation is a recommendation, not a
// substitution: no hard fail (refusal still present).
func TestJudgeNoAnswerStrictRefusalWithFramedCitation(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:  NoAnswerStrictNotFound,
		Answer:    "库内没有找到该作品；如果你对星际题材感兴趣，可以看看这篇通用教程。",
		Citations: []AnswerEvalCitation{citation(5)},
	})
	if result.HardFail || !result.Pass {
		t.Fatalf("refusal + framed citation = %+v, want pass", result)
	}
}

// H6 related_recommendation_allowed: honest disclaimer + real similar works
// passes.
func TestJudgeNoAnswerRelatedPass(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:        NoAnswerRelatedRecommendationOK,
		Answer:          "没有完全对应的作品，不过《星际mod入门》是相近的模组教程，值得一看。",
		Citations:       []AnswerEvalCitation{citation(5)},
		AcceptableIDs:   []int64{5},
		KnownContentIDs: map[int64]bool{5: true},
	})
	if !result.Pass || !result.HonestDisclaimer || result.HardFail {
		t.Fatalf("related pass = %+v", result)
	}
}

// related hard fail: recommending without the no-exact-match disclaimer
// passes a similar work off as the precise answer.
func TestJudgeNoAnswerRelatedFakeExact(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:        NoAnswerRelatedRecommendationOK,
		Answer:          "《星际mod入门》就是你要找的教程，涵盖了全部内容。",
		Citations:       []AnswerEvalCitation{citation(5)},
		AcceptableIDs:   []int64{5},
		KnownContentIDs: map[int64]bool{5: true},
	})
	if !result.HardFail || !result.FakeExactClaim || result.Pass {
		t.Fatalf("related fake exact = %+v, want hard fail", result)
	}
}

// related hard fail: recommendation of nonexistent content.
func TestJudgeNoAnswerRelatedFakeRecommendation(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:        NoAnswerRelatedRecommendationOK,
		Answer:          "没有完全对应的作品，看看《不存在的mod大全》吧。",
		Citations:       []AnswerEvalCitation{citation(42)},
		AcceptableIDs:   []int64{5},
		KnownContentIDs: map[int64]bool{5: true}, // 42 not in the universe
	})
	if !result.HardFail || !result.FakeRecommendation {
		t.Fatalf("fake recommendation = %+v, want hard fail", result)
	}
}

// must_not_claim violations are hard fails under both strategies and with
// 《》-insensitive matching.
func TestJudgeNoAnswerMustNotClaim(t *testing.T) {
	for _, strategy := range []string{NoAnswerStrictNotFound, NoAnswerRelatedRecommendationOK} {
		result := JudgeNoAnswer(NoAnswerJudgeInput{
			Strategy:     strategy,
			Answer:       "没有完全对应的作品。不过《星海遗响》第二部讲述了……",
			MustNotClaim: []string{"星海遗响"},
		})
		if !result.Fabricated || !result.HardFail || result.Pass {
			t.Fatalf("strategy %s must_not_claim = %+v, want hard fail", strategy, result)
		}
		if len(result.ClaimedTitles) != 1 || result.ClaimedTitles[0] != "星海遗响" {
			t.Fatalf("claimed titles = %v", result.ClaimedTitles)
		}
	}

	// case-insensitive Latin claim matching
	result := JudgeNoAnswer(NoAnswerJudgeInput{
		Strategy:     NoAnswerRelatedRecommendationOK,
		Answer:       "没有完全对应的作品。check out Starfall Chronicles for more.",
		MustNotClaim: []string{"starfall chronicles"},
	})
	if !result.Fabricated {
		t.Fatalf("latin claim = %+v, want fabricated", result)
	}
}

// unknown strategies fail closed.
func TestJudgeNoAnswerUnknownStrategy(t *testing.T) {
	result := JudgeNoAnswer(NoAnswerJudgeInput{Strategy: "maybe", Answer: "找不到"})
	if result.Pass || result.HardFail || result.Reason == "" {
		t.Fatalf("unknown strategy = %+v, want a data error", result)
	}
}
