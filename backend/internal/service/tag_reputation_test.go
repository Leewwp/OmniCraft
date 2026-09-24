package service

// #659（用户裁决 B）：未接线的 tag_recognized 声誉奖励规则整体移除——
// 生产组合根从未注入 TagService 声誉依赖（#657 勘误），规则删除不改变
// 生产奖励流。本文件原为 T33「+1 信誉分」接线测试，替换为无奖励特征：
// 批准他人 add 建议后标签生效、建议者声誉与 reputation_logs 不变。
// 历史 tag_recognized 日志与前端 reason 文案保留（历史兼容显示）。

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func setupT33TagReputationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.ContentTag{}, &model.TagSuggestion{}, &model.ReputationLog{}))
	return db
}

func t33Suggestion(t *testing.T, db *gorm.DB, suggesterID, authorID int64, action string) *TagService {
	t.Helper()
	if suggesterID == authorID {
		author := model.User{ID: 1, Email: "t33-author@example.test", Username: "t33-author", PasswordHash: "hash", Reputation: 10}
		require.NoError(t, db.Create(&author).Error)
	} else {
		author := model.User{ID: 1, Email: "t33-author@example.test", Username: "t33-author", PasswordHash: "hash", Reputation: 10}
		suggester := model.User{ID: 2, Email: "t33-suggester@example.test", Username: "t33-suggester", PasswordHash: "hash", Reputation: 10}
		require.NoError(t, db.Create(&author).Error)
		require.NoError(t, db.Create(&suggester).Error)
	}

	content := model.ContentItem{ID: 10, Title: "T33 内容", AuthorID: authorID, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&content).Error)

	sg := model.TagSuggestion{ContentItemID: content.ID, UserID: suggesterID, Tag: "奇幻", Action: action, Status: "pending"}
	require.NoError(t, db.Create(&sg).Error)

	return NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil)
}

func t33ReputationLogs(t *testing.T, db *gorm.DB, userID int64) []model.ReputationLog {
	t.Helper()
	var logs []model.ReputationLog
	require.NoError(t, db.Where("user_id = ?", userID).Find(&logs).Error)
	return logs
}

func TestApproveTagSuggestionAppliesTagWithoutAnyReputationAward(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 2, 1, "add") // 建议者 ≠ 作者

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.EqualValues(t, "pending", sg.Status)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1)) // 作者批准

	// 标签生效主语义保持：建议转 approved、标签落内容。
	var after model.TagSuggestion
	require.NoError(t, db.First(&after, sg.ID).Error)
	require.EqualValues(t, "approved", after.Status)
	var contentTags []model.ContentTag
	require.NoError(t, db.Where("content_item_id = ?", after.ContentItemID).Find(&contentTags).Error)
	found := false
	for _, ct := range contentTags {
		if ct.Tag == "奇幻" {
			found = true
		}
	}
	require.True(t, found, "批准 add 建议后标签必须写入内容")

	// #659 特征：建议者声誉与 reputation_logs 均不变（奖励规则已移除）。
	require.Empty(t, t33ReputationLogs(t, db, 2), "批准他人标签建议不得再写声誉日志")
	var suggester model.User
	require.NoError(t, db.First(&suggester, 2).Error)
	require.EqualValues(t, 10, suggester.Reputation, "建议者声誉分保持不变")
}

func TestApproveOwnTagSuggestionStillAppliesWithoutAward(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 1, 1, "add") // 建议者 == 作者

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1))

	require.Empty(t, t33ReputationLogs(t, db, 1), "自建议同样不产生声誉日志")
}

func TestApproveRemoveSuggestionStillAppliesWithoutAward(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 2, 1, "remove")

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1))

	require.Empty(t, t33ReputationLogs(t, db, 2), "remove 建议不产生声誉日志")
}
