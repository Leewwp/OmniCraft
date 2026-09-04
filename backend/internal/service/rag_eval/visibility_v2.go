package rageval

import (
	"fmt"
	"sort"
	"strings"
)

// Visibility leak surfaces for the v2 harness (contract §8-H7). The v1
// runner measured only the retrieval-id surface; the v2 gate requires all
// four surfaces to be zero for every visibility case under both principals.

// ForbiddenDoc is the text identity of one restricted (non-public) document
// used for mention detection on the citation/answer/tool surfaces.
type ForbiddenDoc struct {
	ContentID int64
	Title     string
	Summary   string
}

// ToolStepRecord is the harness-side mirror of one SSE tool-step event
// (AgentToolExecution): server-derived name and display-safe argument
// summary, no raw provider payloads.
type ToolStepRecord struct {
	Name        string
	ArgsSummary string
}

// VisibilityLeakInput carries the four produced surfaces of one case run.
type VisibilityLeakInput struct {
	ForbiddenIDs []int64
	ForbiddenDoc []ForbiddenDoc
	// Surface 1: retrieval result ids (the v1 VisibilityLeaks surface).
	RetrievedIDs []int64
	// Surface 2: citations the answer produced.
	Citations []AnswerEvalCitation
	// Surface 3: the answer body text.
	Answer string
	// Surface 4: the SSE tool-step event stream the client saw.
	ToolSteps []ToolStepRecord
}

// VisibilityLeakReport is the per-case leak accounting. AllZero is the hard
// gate: any leak on any surface fails the case under both principals.
type VisibilityLeakReport struct {
	RetrievalIDs   []int64 `json:"retrieval_ids,omitempty"`
	Citations      []int64 `json:"citations,omitempty"`
	AnswerMentions []int64 `json:"answer_mentions,omitempty"`
	ToolSteps      []int64 `json:"tool_steps,omitempty"`
	AllZero        bool    `json:"all_zero"`
}

// EvaluateVisibilityLeaks scores the four leak surfaces:
//  1. retrieval result ids — restricted ids returned by the retriever;
//  2. citations — restricted ids cited by the answer;
//  3. answer body mentions — restricted title/summary (normalised) or a
//     bounded numeric id reference appearing in the answer text;
//  4. tool-step events — the same text match over the display-safe argument
//     summaries the client received.
func EvaluateVisibilityLeaks(input VisibilityLeakInput) VisibilityLeakReport {
	forbidden := make(map[int64]bool, len(input.ForbiddenIDs))
	for _, id := range input.ForbiddenIDs {
		forbidden[id] = true
	}

	report := VisibilityLeakReport{
		RetrievalIDs: VisibilityLeaks(input.RetrievedIDs, forbidden),
	}
	for _, c := range input.Citations {
		if forbidden[c.ContentID] {
			report.Citations = append(report.Citations, c.ContentID)
		}
	}
	report.Citations = dedupeSorted(report.Citations)

	docsByID := make(map[int64]ForbiddenDoc, len(input.ForbiddenDoc))
	for _, doc := range input.ForbiddenDoc {
		docsByID[doc.ContentID] = doc
	}
	// the id text match needs no doc metadata; title/summary matching runs
	// whenever the restricted identity is known
	for _, id := range input.ForbiddenIDs {
		if !forbidden[id] {
			continue
		}
		doc, hasDoc := docsByID[id]
		if mentionsBoundedID(input.Answer, id) || (hasDoc && mentionsForbiddenText(input.Answer, doc)) {
			report.AnswerMentions = append(report.AnswerMentions, id)
		}
	}
	report.AnswerMentions = dedupeSorted(report.AnswerMentions)

	for _, step := range input.ToolSteps {
		for _, id := range input.ForbiddenIDs {
			if !forbidden[id] || alreadyReported(report.ToolSteps, id) {
				continue
			}
			doc, hasDoc := docsByID[id]
			if mentionsBoundedID(step.ArgsSummary, id) || (hasDoc && mentionsForbiddenText(step.ArgsSummary, doc)) {
				report.ToolSteps = append(report.ToolSteps, id)
			}
		}
	}
	report.ToolSteps = dedupeSorted(report.ToolSteps)

	report.AllZero = len(report.RetrievalIDs) == 0 &&
		len(report.Citations) == 0 &&
		len(report.AnswerMentions) == 0 &&
		len(report.ToolSteps) == 0
	return report
}

func alreadyReported(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func dedupeSorted(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := ids[:1]
	for _, id := range ids[1:] {
		if id != out[len(out)-1] {
			out = append(out, id)
		}
	}
	return out
}

// mentionsForbiddenText checks whether the restricted document's title or
// summary appears in the text after claim normalisation (case fold,
// whitespace collapse, outer 《》 stripping), so 《机密文档》 matches
// "关于《机密文档》的讨论".
func mentionsForbiddenText(text string, doc ForbiddenDoc) bool {
	for _, candidate := range []string{doc.Title, doc.Summary} {
		candidate = strings.TrimSpace(candidate)
		if len(candidate) < 2 {
			continue
		}
		if containsNormalizedText(text, candidate) {
			return true
		}
	}
	return false
}

// mentionsBoundedID checks whether the decimal form of the restricted id
// appears as a standalone token (non-digit or boundary on both sides), so
// content id 1006 matches "#1006" or "内容 1006" but not "41006".
func mentionsBoundedID(text string, id int64) bool {
	needle := fmt.Sprintf("%d", id)
	start := 0
	for {
		idx := strings.Index(text[start:], needle)
		if idx < 0 {
			return false
		}
		at := start + idx
		beforeOK := at == 0 || !isASCIIDigit(rune(text[at-1]))
		after := at + len(needle)
		afterOK := after >= len(text) || !isASCIIDigit(rune(text[after]))
		if beforeOK && afterOK {
			return true
		}
		start = at + 1
	}
}

func isASCIIDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
