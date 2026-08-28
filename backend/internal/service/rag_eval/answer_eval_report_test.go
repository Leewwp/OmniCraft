package rageval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitAnswerSentences(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   []string
	}{
		{name: "cjk marks", answer: "这是第一句。这是第二句！第三句呢？", want: []string{"这是第一句", "这是第二句", "第三句呢"}},
		{name: "latin marks", answer: "First sentence. Second one! A third?", want: []string{"First sentence", "Second one", "A third"}},
		{name: "decimal kept", answer: "版本号是 3.5 发布了。下一句。", want: []string{"版本号是 3.5 发布了", "下一句"}},
		{name: "newline splits", answer: "第一行\n第二行", want: []string{"第一行", "第二行"}},
		{name: "empty", answer: "  ", want: []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitAnswerSentences(tc.answer)
			if len(got) != len(tc.want) {
				t.Fatalf("sentences = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("sentences = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestScoreAnswerCase(t *testing.T) {
	evidence := []string{"Blender 插件安装教程：下载对应系统的安装包，双击安装即可完成。安装完成后重启 Blender。"}
	t.Run("verbatim answer fully grounded", func(t *testing.T) {
		answer := "下载对应系统的安装包，双击安装即可完成。安装完成后重启 Blender。"
		g, r, sentences := ScoreAnswerCase(answer, "Blender 插件安装教程", evidence)
		if g == nil || *g != 1.0 {
			t.Fatalf("groundedness = %v, want 1.0", g)
		}
		if r == nil || *r <= 0 {
			t.Fatalf("answer relevance = %v, want > 0", r)
		}
		if sentences != 2 {
			t.Fatalf("sentences = %d, want 2", sentences)
		}
	})
	t.Run("paraphrased answer scores below one", func(t *testing.T) {
		answer := "去官网下载最新版本安装就好啦，装完记得重新打开软件哦。"
		g, _, _ := ScoreAnswerCase(answer, "Blender 插件安装教程", evidence)
		if g == nil || *g >= 1.0 {
			t.Fatalf("groundedness = %v, want < 1.0 for paraphrase", g)
		}
	})
	t.Run("no evidence yields nil", func(t *testing.T) {
		g, r, sentences := ScoreAnswerCase("随便说说", "问题", nil)
		if g != nil || r != nil || sentences != 0 {
			t.Fatalf("want nil metrics with no evidence, got g=%v r=%v n=%d", g, r, sentences)
		}
	})
	t.Run("empty answer yields nil", func(t *testing.T) {
		g, r, _ := ScoreAnswerCase("", "问题", evidence)
		if g != nil || r != nil {
			t.Fatalf("want nil metrics with empty answer, got g=%v r=%v", g, r)
		}
	})
}

func answerEvalFixtureCases() []AnswerEvalCaseResult {
	g1 := 0.5
	g2 := 1.0
	r1 := 0.4
	ft1 := int64(1200)
	ft2 := int64(2400)
	pt1 := 900
	ct1 := 200
	pt2 := 1100
	ct2 := 300
	return []AnswerEvalCaseResult{
		{CaseKey: "a", Status: AnswerStatusAnswered, Attempts: 2, DirectAnswers: 1, Groundedness: &g1, AnswerRelevance: &r1, FirstTokenMs: &ft1, TotalMs: 3000, PromptTokens: &pt1, CompletionTokens: &ct1},
		{CaseKey: "b", Status: AnswerStatusAnswered, Attempts: 1, Groundedness: &g2, FirstTokenMs: &ft2, TotalMs: 5000, PromptTokens: &pt2, CompletionTokens: &ct2},
		{CaseKey: "c", Status: AnswerStatusNoEvidence, Attempts: 2, DirectAnswers: 2},
		{CaseKey: "d", Status: AnswerStatusDegraded, Degraded: true, Attempts: 1},
		{CaseKey: "e", Status: AnswerStatusProvError, ErrorCode: "AGENT_PROVIDER_TIMEOUT", Attempts: 1},
	}
}

func TestBuildAnswerEvalReportEffectiveDenominators(t *testing.T) {
	report := BuildAnswerEvalReport(AnswerEvalMetadata{Provider: "minimax"}, answerEvalFixtureCases())
	s := report.Summary
	if s.Cases != 5 || s.Answered != 2 || s.NoEvidence != 1 || s.Degraded != 1 || s.ProviderErrors != 1 {
		t.Fatalf("bucket counts wrong: %+v", s)
	}
	if s.Groundedness == nil || *s.Groundedness < 0.749 || *s.Groundedness > 0.751 {
		t.Fatalf("groundedness mean = %v, want 0.75 over answered-with-evidence cases", s.Groundedness)
	}
	if s.AnswerRelevance == nil || *s.AnswerRelevance < 0.399 || *s.AnswerRelevance > 0.401 {
		t.Fatalf("relevance mean = %v, want 0.4 (only one scoreable case)", s.AnswerRelevance)
	}
	if s.FirstTokenP50Ms == nil || s.FirstTokenP95Ms == nil {
		t.Fatalf("first-token percentiles missing: %+v", s)
	}
	if *s.FirstTokenP50Ms > *s.FirstTokenP95Ms {
		t.Fatalf("p50 %v > p95 %v", *s.FirstTokenP50Ms, *s.FirstTokenP95Ms)
	}
	if s.TotalP50Ms == nil || *s.TotalP50Ms <= 0 {
		t.Fatalf("total p50 missing: %+v", s)
	}
	if s.PromptTokensTotal != 2000 || s.CompletionTokensTotal != 500 {
		t.Fatalf("token totals wrong: %+v", s)
	}
	if s.TokensPerAnswerMean == nil || *s.TokensPerAnswerMean != 1250 {
		t.Fatalf("tokens per answer = %v, want 1250", s.TokensPerAnswerMean)
	}
	if s.TotalAttempts != 7 || s.DirectAnswerAttempts != 3 {
		t.Fatalf("attempt accounting wrong: total=%d direct=%d", s.TotalAttempts, s.DirectAnswerAttempts)
	}
	if s.DirectAnswerRate == nil || *s.DirectAnswerRate < 0.42 || *s.DirectAnswerRate > 0.43 {
		t.Fatalf("direct answer rate = %v, want ~0.429", s.DirectAnswerRate)
	}
}

func TestBuildAnswerEvalReportEmptyStaysNil(t *testing.T) {
	report := BuildAnswerEvalReport(AnswerEvalMetadata{}, []AnswerEvalCaseResult{
		{CaseKey: "a", Status: AnswerStatusNoEvidence},
	})
	if report.Summary.Groundedness != nil || report.Summary.AnswerRelevance != nil || report.Summary.FirstTokenP50Ms != nil {
		t.Fatalf("empty buckets must stay nil: %+v", report.Summary)
	}
}

func TestWriteAnswerEvalReportContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "answer_eval.json")
	report := BuildAnswerEvalReport(AnswerEvalMetadata{
		RanAt: "2026-08-28T00:00:00Z", Provider: "minimax", ChatModel: "MiniMax-M3",
		Note: "local measurement",
	}, answerEvalFixtureCases())
	if err := WriteAnswerEvalReport(path, report); err != nil {
		t.Fatalf("write report: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("report is not valid JSON: %v", err)
	}
	if decoded["schema_version"] != answerEvalSchemaVersion {
		t.Fatalf("schema_version = %v, want %v", decoded["schema_version"], answerEvalSchemaVersion)
	}
	// Redaction contract: the artifact struct has no credential fields, and the
	// serialized output must never contain key-shaped material.
	lower := strings.ToLower(string(data))
	for _, forbidden := range []string{"api_key", "apikey", "authorization", "bearer ", "sk-"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("report contains forbidden credential marker %q", forbidden)
		}
	}
}
