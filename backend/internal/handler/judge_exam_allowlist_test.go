package handler

// T37（FIX-36a / F-100）：GetExam 序列化改 allowlist——只透出
// id/content_type/prompt/options，杜绝 correct_key / votes_* / explanation
// 等答案性字段下发（调度器题内嵌投票数可机械推答案）。

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/model"
)

func TestT37SanitizeExamQuestionAllowlist(t *testing.T) {
	data, err := json.Marshal(map[string]interface{}{
		"question":      "根据多数投票意见，以下内容是否违规？",
		"options":       map[string]string{"A": "违规（应下架）", "B": "不违规（应保留）"},
		"correct_key":   "A",
		"votes_approve": 3,
		"votes_reject":  1,
		"explanation":   "解析：多数派认定违规，正确答案 A",
	})
	require.NoError(t, err)

	out := sanitizeExamQuestion(model.JudgeQuestion{ID: 42, ContentType: "article", QuestionData: data})
	payload, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(payload)

	require.NotContains(t, s, "correct_key")
	require.NotContains(t, s, "votes_approve")
	require.NotContains(t, s, "votes_reject")
	require.NotContains(t, s, "explanation")
	require.NotContains(t, s, "多数派认定违规", "explanation 明文不得下发")

	// 调度器题干字段 question 兼容输出为 prompt；选项正常透出。
	require.Contains(t, s, "prompt")
	require.Contains(t, s, "根据多数投票意见，以下内容是否违规？")
	require.Contains(t, s, "违规（应下架）")
}

func TestT37SanitizeExamQuestionPrefersPromptField(t *testing.T) {
	data, err := json.Marshal(map[string]interface{}{
		"prompt":      "以下哪种行为属于滥用标签？",
		"options":     map[string]string{"A": "批量乱打标签", "B": "规范使用"},
		"correct_key": "A",
	})
	require.NoError(t, err)

	out := sanitizeExamQuestion(model.JudgeQuestion{ID: 7, ContentType: "article", QuestionData: data})
	payload, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(payload)

	require.Contains(t, s, "以下哪种行为属于滥用标签？")
	require.NotContains(t, s, "correct_key")
}

func TestT37SanitizeExamQuestionMalformedData(t *testing.T) {
	out := sanitizeExamQuestion(model.JudgeQuestion{ID: 9, ContentType: "article", QuestionData: []byte("{bad json")})
	payload, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(payload)

	require.Contains(t, s, `"id":9`)
	require.NotContains(t, s, "correct_key")
}
