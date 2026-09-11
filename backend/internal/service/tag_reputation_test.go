package service

// T33（FIX-37）③：标签建议被认可 → 建议者 +1 信誉分（business-rules 承诺接线）。
// 仅 action=add 且建议者≠内容作者时加分（作者自加标签不自我激励）。

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
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.TagSuggestion{}, &model.ReputationLog{}))
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

	svc := NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil)
	svc.SetReputationService(NewReputationService(db))
	return svc
}

func t33ReputationLogs(t *testing.T, db *gorm.DB, userID int64) []model.ReputationLog {
	t.Helper()
	var logs []model.ReputationLog
	require.NoError(t, db.Where("user_id = ?", userID).Find(&logs).Error)
	return logs
}

func TestApproveTagSuggestionAwardsSuggester(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 2, 1, "add")

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1)) // 作者批准

	logs := t33ReputationLogs(t, db, 2)
	require.Len(t, logs, 1, "认可他人标签建议必须给建议者 +1 信誉分")
	require.Equal(t, "tag_recognized", logs[0].Reason)
	require.EqualValues(t, 1, logs[0].Delta)
	require.NotNil(t, logs[0].RelatedID)
	require.EqualValues(t, sg.ID, *logs[0].RelatedID)
}

func TestApproveTagSuggestionNoAwardForSelfSuggestion(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 1, 1, "add") // 建议者==作者

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1))

	require.Empty(t, t33ReputationLogs(t, db, 1), "作者自建议不得自我加分")
}

func TestApproveTagSuggestionNoAwardForRemoveAction(t *testing.T) {
	db := setupT33TagReputationDB(t)
	svc := t33Suggestion(t, db, 2, 1, "remove")

	var sg model.TagSuggestion
	require.NoError(t, db.First(&sg).Error)
	require.NoError(t, svc.ApproveTagSuggestion(sg.ID, 1))

	require.Empty(t, t33ReputationLogs(t, db, 2), "remove 建议不触发加分")
}
