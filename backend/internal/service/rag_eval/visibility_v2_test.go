package rageval

import "testing"

var leakDocs = []ForbiddenDoc{
	{ContentID: 1006, Title: "《未公开的机密mod》", Summary: "内部测试用的私有模组文档"},
	{ContentID: 1007, Title: "Private Guide", Summary: "an unpublished draft"},
}

// H7: a clean run leaks on none of the four surfaces.
func TestEvaluateVisibilityLeaksAllZero(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006, 1007},
		ForbiddenDoc: leakDocs,
		RetrievedIDs: []int64{2001, 2002},
		Citations:    []AnswerEvalCitation{citation(2001)},
		Answer:       "这是《公开教程》的安装步骤……",
		ToolSteps:    []ToolStepRecord{{Name: "search_content", ArgsSummary: "query=公开教程 hits=2"}},
	})
	if !report.AllZero {
		t.Fatalf("clean run = %+v, want all zero", report)
	}
}

func TestEvaluateVisibilityLeaksRetrievalSurface(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006},
		RetrievedIDs: []int64{2001, 1006},
	})
	if report.AllZero || len(report.RetrievalIDs) != 1 || report.RetrievalIDs[0] != 1006 {
		t.Fatalf("retrieval leak = %+v", report)
	}
}

func TestEvaluateVisibilityLeaksCitationSurface(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006},
		Citations:    []AnswerEvalCitation{citation(1006), citation(2001)},
	})
	if report.AllZero || len(report.Citations) != 1 || report.Citations[0] != 1006 {
		t.Fatalf("citation leak = %+v", report)
	}
}

// answer-body mention: title match survives 《》 stripping; the bounded id
// match catches a bare "#1006" but not "41006".
func TestEvaluateVisibilityLeaksAnswerSurface(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006},
		ForbiddenDoc: leakDocs,
		Answer:       "虽然没有直接结果，但《未公开的机密mod》似乎相关。",
	})
	if report.AllZero || len(report.AnswerMentions) != 1 || report.AnswerMentions[0] != 1006 {
		t.Fatalf("answer title leak = %+v", report)
	}

	report = EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006},
		Answer:       "参见内容 1006 了解详情。",
	})
	if report.AllZero || len(report.AnswerMentions) != 1 {
		t.Fatalf("bounded id leak = %+v", report)
	}

	report = EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006},
		Answer:       "编号 41006 与 10061 都无关。",
	})
	if !report.AllZero {
		t.Fatalf("unbounded digits = %+v, want no leak", report)
	}

	// summary text also counts as a mention surface
	report = EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1007},
		ForbiddenDoc: leakDocs,
		Answer:       "相关内容是 an unpublished draft（内部草稿）。",
	})
	if report.AllZero || len(report.AnswerMentions) != 1 || report.AnswerMentions[0] != 1007 {
		t.Fatalf("summary leak = %+v", report)
	}
}

// tool-step surface: the display-safe arg summary the client saw carries the
// restricted title.
func TestEvaluateVisibilityLeaksToolSurface(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1007},
		ForbiddenDoc: leakDocs,
		ToolSteps: []ToolStepRecord{
			{Name: "search_content", ArgsSummary: "query=private guide hits=1"},
		},
	})
	if report.AllZero || len(report.ToolSteps) != 1 || report.ToolSteps[0] != 1007 {
		t.Fatalf("tool-step leak = %+v", report)
	}

	clean := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1007},
		ForbiddenDoc: leakDocs,
		ToolSteps:    []ToolStepRecord{{Name: "search_content", ArgsSummary: "query=公开教程 hits=3"}},
	})
	if !clean.AllZero {
		t.Fatalf("clean tool steps = %+v", clean)
	}
}

// multiple surfaces leak at once and dedupe by id.
func TestEvaluateVisibilityLeaksMultiSurface(t *testing.T) {
	report := EvaluateVisibilityLeaks(VisibilityLeakInput{
		ForbiddenIDs: []int64{1006, 1007},
		ForbiddenDoc: leakDocs,
		RetrievedIDs: []int64{1006, 1006, 1007},
		Citations:    []AnswerEvalCitation{citation(1007)},
		Answer:       "《未公开的机密mod》相关内容见 1007。",
		ToolSteps:    []ToolStepRecord{{Name: "search_content", ArgsSummary: "query=未公开的机密mod hits=1"}},
	})
	if report.AllZero {
		t.Fatal("multi-surface run must leak")
	}
	if len(report.RetrievalIDs) != 2 || len(report.Citations) != 1 || len(report.AnswerMentions) != 2 || len(report.ToolSteps) != 1 {
		t.Fatalf("multi-surface report = %+v", report)
	}
}
