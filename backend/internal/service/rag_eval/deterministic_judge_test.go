package rageval

import (
	"strings"
	"testing"
)

func TestJudgeDeterministicAssertionsPassWithPunctuationVariance(t *testing.T) {
	answer := "《全职高手》战术楼的关键点：拆解站位钓饵。"
	assertions := []string{"战术楼的关键点：拆解站位钓饵"}
	got := JudgeDeterministicAssertions(answer, assertions)
	if !got.Pass {
		t.Fatalf("full-width colon assertion must pass against half-width answer: %+v", got.Results)
	}
	// reversed direction: assertion uses half-width, answer full-width
	got = JudgeDeterministicAssertions("关键点：拆解", []string{"关键点:拆解"})
	if !got.Pass {
		t.Fatalf("half-width assertion must pass against full-width answer: %+v", got.Results)
	}
}

func TestJudgeDeterministicAssertionsCaseAndWhitespaceFold(t *testing.T) {
	answer := "The  Inkblot   Test：PASS"
	got := JudgeDeterministicAssertions(answer, []string{"the inkblot test:pass"})
	if !got.Pass {
		t.Fatalf("case/whitespace folding failed: %+v", got.Results)
	}
}

func TestJudgeDeterministicAssertionsShortLatinNeedsBoundaries(t *testing.T) {
	// "ink" inside "thinking" is a substring, not a standalone token
	got := JudgeDeterministicAssertions("I was thinking about it", []string{"ink"})
	if got.Pass {
		t.Fatal("short latin assertion must not match as a substring inside a longer word")
	}
	got = JudgeDeterministicAssertions("the ink dried", []string{"ink"})
	if !got.Pass {
		t.Fatalf("standalone short token must match: %+v", got.Results)
	}
}

func TestJudgeDeterministicAssertionsCJKSubstring(t *testing.T) {
	got := JudgeDeterministicAssertions("这是一段关于钓饵站位的讨论", []string{"钓饵站位"})
	if !got.Pass {
		t.Fatalf("CJK substring match failed: %+v", got.Results)
	}
	got = JudgeDeterministicAssertions("完全不相关的内容", []string{"钓饵站位"})
	if got.Pass {
		t.Fatal("absent CJK assertion must fail")
	}
}

func TestJudgeDeterministicAssertionsInvalidTooShort(t *testing.T) {
	got := JudgeDeterministicAssertions("任何答案", []string{"a"})
	if got.Pass || got.Invalid != 1 {
		t.Fatalf("single-rune assertion must be invalid and fail closed: %+v", got)
	}
	got = JudgeDeterministicAssertions("答案", []string{"  "})
	if got.Pass || got.Invalid != 1 {
		t.Fatalf("whitespace-only assertion must be invalid: %+v", got)
	}
}

func TestJudgeDeterministicAssertionsInvalidExcludedFromPass(t *testing.T) {
	got := JudgeDeterministicAssertions("答案里有摩拉换算的完整表述", []string{"文", "摩拉换算"})
	if !got.Pass || got.Invalid != 1 || got.Passed != 1 {
		t.Fatalf("invalid assertion must be excluded, not fail the case: %+v", got)
	}
	allInvalid := JudgeDeterministicAssertions("任何答案", []string{"文"})
	if allInvalid.Pass {
		t.Fatal("all-invalid case is unjudgeable, not pass")
	}
}

func TestJudgeDeterministicAssertionsEmptyAnswerFails(t *testing.T) {
	got := JudgeDeterministicAssertions("   ", []string{"任何断言"})
	if got.Pass {
		t.Fatal("empty answer must fail every assertion")
	}
	if !strings.Contains(got.Results[0].Reason, "empty answer") {
		t.Fatalf("unexpected reason %q", got.Results[0].Reason)
	}
}

func TestJudgeDeterministicAssertionsNoAssertions(t *testing.T) {
	if got := JudgeDeterministicAssertions("answer", nil); !got.Pass {
		t.Fatal("no assertions means pass (nothing to violate)")
	}
}

func TestNormalizeAssertionText(t *testing.T) {
	got := NormalizeAssertionText("Ａｂｃ　Ｔｅｓｔ：Ｘ")
	// full-width ASCII folds to half-width (width folding is part of the
	// contract, matching the #319 half/full-width colon lesson).
	want := "abc test:x"
	if got != want {
		t.Fatalf("normalize = %q, want %q", got, want)
	}
}
