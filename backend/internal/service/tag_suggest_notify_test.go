package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// T49（FIX-22c）：标签建议提交成功后通知内容作者（channel=system、
// type=tag_suggestion、target=content、body 携带 tag 与内容标题）；
// 作者本人提建议不通知。限流关闭（rdb=nil）时行为不变。

type t49CaptureProducer struct {
	mu        sync.Mutex
	publishes []map[string]interface{}
}

func (p *t49CaptureProducer) Publish(_ context.Context, _ string, payload []byte) error {
	var decoded map[string]interface{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishes = append(p.publishes, decoded)
	return nil
}

func (p *t49CaptureProducer) tagSuggestionNotifies() []map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]interface{}, 0)
	for _, pub := range p.publishes {
		if pub["type"] == "tag_suggestion" {
			out = append(out, pub)
		}
	}
	return out
}

func setupT49DB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.TagSuggestion{}))
	return db
}

func TestSuggestTagNotifiesContentAuthor(t *testing.T) {
	db := setupT49DB(t)

	author := model.User{ID: 1, Email: "t49-author@example.test", Username: "t49-author", PasswordHash: "hash", Reputation: 10}
	suggester := model.User{ID: 2, Email: "t49-suggester@example.test", Username: "t49-suggester", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	require.NoError(t, db.Create(&suggester).Error)

	content := model.ContentItem{ID: 10, Title: "T49 内容", AuthorID: author.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&content).Error)

	producer := &t49CaptureProducer{}
	svc := NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil)
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	svc.SetNotificationService(notifSvc)

	require.NoError(t, svc.SuggestTag(content.ID, suggester.ID, "奇幻", "add"))

	notifies := producer.tagSuggestionNotifies()
	require.Len(t, notifies, 1, "标签建议提交成功应通知内容作者")
	require.EqualValues(t, author.ID, notifies[0]["user_id"])
	require.Equal(t, "system", notifies[0]["channel"])
	require.Equal(t, "content", notifies[0]["target_type"])
	require.EqualValues(t, content.ID, notifies[0]["target_id"])
	require.Contains(t, notifies[0]["body"], "奇幻")
	require.Contains(t, notifies[0]["body"], "T49 内容")
	require.EqualValues(t, suggester.ID, notifies[0]["sender_id"], "建议者作为 sender")
}

func TestSuggestTagByAuthorSelfDoesNotNotify(t *testing.T) {
	db := setupT49DB(t)

	author := model.User{ID: 1, Email: "t49-self@example.test", Username: "t49-self", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	content := model.ContentItem{ID: 11, Title: "T49 自建议", AuthorID: author.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&content).Error)

	producer := &t49CaptureProducer{}
	svc := NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil)
	notifSvc := NewNotificationService(repository.NewNotificationRepository(db))
	notifSvc.SetQueueProducer(producer)
	svc.SetNotificationService(notifSvc)

	require.NoError(t, svc.SuggestTag(content.ID, author.ID, "奇幻", "add"))
	require.Empty(t, producer.tagSuggestionNotifies(), "作者本人提建议不应产生通知")
}

func TestSuggestTagWithoutNotificationServiceStillSucceeds(t *testing.T) {
	db := setupT49DB(t)

	author := model.User{ID: 1, Email: "t49-noauthor@example.test", Username: "t49-noauthor", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)
	content := model.ContentItem{ID: 12, Title: "T49 无通知服务", AuthorID: author.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&content).Error)

	// 未装配 notifSvc（本地/测试路径）时建议照常落库，不 panic
	svc := NewTagService(repository.NewTagRepository(db), repository.NewContentRepository(db), nil, nil)
	require.NoError(t, svc.SuggestTag(content.ID, 1, "冒险", "add"))

	var count int64
	require.NoError(t, db.Model(&model.TagSuggestion{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
