package handler

// run-1 审计 #13：申诉提交校验目标存在但不比对归属——任何人可对他人内容/
// 评论提申诉；单一 pending 不变量此前按 user_id 维度查重（HasPendingAppeal），
// 退化为按调用者而非按目标。修复后：content/comment 申诉仅目标作者本人可提
//（403 FORBIDDEN），查重按 target 维度（同目标最多一个 pending，409）。

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
)

func TestSubmitAppealRejectsForeignTarget(t *testing.T) {
	appealRouter, _, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	author := t31SeedUser(t, db, 2, false)
	caller := t31SeedUser(t, db, 3, false)
	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"

	content := model.ContentItem{AuthorID: author.ID, Title: "foreign content", Status: "banned"}
	require.NoError(t, db.Create(&content).Error)
	comment := model.Comment{AuthorID: author.ID, Body: "foreign comment", Status: "hidden"}
	require.NoError(t, db.Create(&comment).Error)

	w := t31SubmitAppeal(t, appealRouter, cfg, caller.ID, `{"target_type":"content","target_id":`+strconv.FormatInt(content.ID, 10)+`,"reason":"not mine"}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "FORBIDDEN")

	w = t31SubmitAppeal(t, appealRouter, cfg, caller.ID, `{"target_type":"comment","target_id":`+strconv.FormatInt(comment.ID, 10)+`,"reason":"not mine"}`)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "FORBIDDEN")

	// 目标本人照常提交（account 分支既有归属绑定不受影响）。
	w = t31SubmitAppeal(t, appealRouter, cfg, author.ID, `{"target_type":"content","target_id":`+strconv.FormatInt(content.ID, 10)+`,"reason":"my content"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestSubmitAppealEnforcesPerTargetSinglePending(t *testing.T) {
	appealRouter, _, db, _, mr := setupT31AppealQualityRouter(t)
	defer mr.Close()
	author := t31SeedUser(t, db, 2, false)
	cfg := &config.Config{}
	cfg.JWT.Secret = "t31-appeal-secret"

	content := model.ContentItem{AuthorID: author.ID, Title: "pending target", Status: "banned"}
	require.NoError(t, db.Create(&content).Error)

	// 目标维度已有 pending（历史行可能来自其他用户——不变量按 target 收口）。
	legacy := model.Appeal{UserID: 999, TargetType: "content", TargetID: content.ID, Reason: "legacy", Status: "pending"}
	require.NoError(t, db.Create(&legacy).Error)

	w := t31SubmitAppeal(t, appealRouter, cfg, author.ID, `{"target_type":"content","target_id":`+strconv.FormatInt(content.ID, 10)+`,"reason":"second"}`)
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "APPEAL_EXISTS")

	// 首个 pending 被处理后同目标可再次申诉。
	require.NoError(t, db.Model(&model.Appeal{}).Where("id = ?", legacy.ID).Update("status", "rejected").Error)
	w = t31SubmitAppeal(t, appealRouter, cfg, author.ID, `{"target_type":"content","target_id":`+strconv.FormatInt(content.ID, 10)+`,"reason":"after resolve"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}
