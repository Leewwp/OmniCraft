package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

// countingLogger 统计命中 content_items 的 SQL 条数（gorm v1.31.2 的 Scan 走
// rows 处理器、不触发 Query callback，故用 logger.Trace 观测批量取数次数）。
type countingLogger struct {
	logger.Interface
	count *int32
}

func (l countingLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if sql, _ := fc(); strings.Contains(sql, "content_items") {
		atomic.AddInt32(l.count, 1)
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// #793：协作邀请深链读侧解析——DB 当前 zone 是唯一导航真源。覆盖票面验收：
// 旧邀请无 content_zone、旧键错误（写时快照过期）、原创与二创、多条重复 ID
// （按页去重一次批量查询）、内容不存在（metadata 原样保留）、历史 metadata
// 不回写、会话参与者校验继续生效。
func TestListMessagesDecoratesCollabInviteNavigation(t *testing.T) {
	router, db := setupMessageRouterWithOptions(t, nil, nil, false)
	contentNavQueries := new(int32)
	db.Logger = countingLogger{Interface: db.Logger, count: contentNavQueries}
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}))

	require.NoError(t, db.Create(&[]model.User{
		{ID: 1, Email: "alice@example.test", PasswordHash: "hash", Username: "alice"},
		{ID: 2, Email: "bob@example.test", PasswordHash: "hash", Username: "bob"},
		{ID: 3, Email: "eve@example.test", PasswordHash: "hash", Username: "eve"},
	}).Error)
	require.NoError(t, db.Create(&[]model.ContentItem{
		{ID: 701, Title: "Original Work", AuthorID: 1, Zone: "original", ContentType: "article", Status: "published"},
		{ID: 702, Title: "Fan Work", AuthorID: 1, Zone: "fanwork", ContentType: "article", Status: "published"},
	}).Error)
	require.NoError(t, db.Create(&model.Conversation{ID: 10}).Error)
	require.NoError(t, db.Create(&[]model.ConversationParticipant{
		{ConversationID: 10, UserID: 1},
		{ConversationID: 10, UserID: 2},
	}).Error)

	base := time.Now().UTC().Add(-time.Hour)
	seedInviteMessage(t, db, 11, "old invite without zone key", model.JSONMap{
		"invite_id": 101, "content_id": 701, "content_title": "Original Work",
		"inviter_id": 1, "inviter_username": "alice",
	}, base)
	seedInviteMessage(t, db, 12, "stale zone key must lose to the database", model.JSONMap{
		"invite_id": 102, "content_id": 701, "content_title": "Original Work",
		"content_zone": "fanwork",
		"inviter_id":   1, "inviter_username": "alice",
	}, base.Add(time.Minute))
	seedInviteMessage(t, db, 13, "fanwork keeps the shared content route", model.JSONMap{
		"invite_id": 103, "content_id": 702, "content_title": "Fan Work",
		"inviter_id": 1, "inviter_username": "alice",
	}, base.Add(2*time.Minute))
	seedInviteMessage(t, db, 14, "duplicate content id on the same page", model.JSONMap{
		"invite_id": 104, "content_id": 702, "content_title": "Fan Work",
		"inviter_id": 1, "inviter_username": "alice",
	}, base.Add(3*time.Minute))
	seedInviteMessage(t, db, 15, "missing target keeps its metadata untouched", model.JSONMap{
		"invite_id": 105, "content_id": 999, "content_title": "Deleted Work",
		"content_zone": "fanwork",
		"inviter_id":   1, "inviter_username": "alice",
	}, base.Add(4*time.Minute))

	queriesBefore := atomic.LoadInt32(contentNavQueries)
	rec := listMessagesAs(t, router, 2, 10)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Messages []struct {
			ID       int64          `json:"id"`
			MsgType  string         `json:"msg_type"`
			Metadata map[string]any `json:"metadata"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	byID := map[int64]map[string]any{}
	for _, m := range body.Messages {
		byID[m.ID] = m.Metadata
	}

	// 旧邀请缺键：原创内容得到 /original/{id}（写侧已不再落 content_zone 快照）。
	require.Equal(t, "original", byID[11]["content_zone"])
	require.Equal(t, "/original/701", byID[11]["content_url"])
	// 旧键错误：以数据库当前 zone 为准，覆盖过期快照。
	require.Equal(t, "original", byID[12]["content_zone"])
	require.Equal(t, "/original/701", byID[12]["content_url"])
	// 二创维持 /content/{id}；同页重复 ID 的两条都装饰。
	require.Equal(t, "fanwork", byID[13]["content_zone"])
	require.Equal(t, "/content/702", byID[13]["content_url"])
	require.Equal(t, "/content/702", byID[14]["content_url"])
	// 目标不存在：恒发 URL+zone 不适用——原 metadata（含旧 content_zone）原样
	// 保留，不加空值键。
	require.Equal(t, "fanwork", byID[15]["content_zone"])
	require.NotContains(t, byID[15], "content_url")
	// 事件快照字段不被装饰破坏。
	require.Equal(t, "Original Work", byID[11]["content_title"])
	require.Equal(t, "alice", byID[11]["inviter_username"])

	// 按页去重、一次批量查询（同页四个邀请去重后一次 IN 查询覆盖全部目标）。
	require.Equal(t, int32(1), atomic.LoadInt32(contentNavQueries)-queriesBefore, "navigation lookup must be one batched query per page")

	// 历史 metadata 不回写：DB 中的消息保持写时内容。
	for _, id := range []int64{11, 12, 15} {
		var stored model.Message
		require.NoError(t, db.First(&stored, id).Error)
		require.NotContains(t, stored.Metadata, "content_url", "message %d must not be rewritten", id)
	}
	var storedOld model.Message
	require.NoError(t, db.First(&storedOld, 11).Error)
	require.NotContains(t, storedOld.Metadata, "content_zone")
	var storedStale model.Message
	require.NoError(t, db.First(&storedStale, 12).Error)
	require.Equal(t, "fanwork", storedStale.Metadata["content_zone"], "stale snapshot must stay as written")
}

// 装饰取数失败（fail-open）：content_items 表缺失时消息列表保持可读，
// 旧回退语义（metadata 原样）不受影响。
func TestListMessagesKeepsContractWhenNavigationLookupFails(t *testing.T) {
	router, db := setupMessageRouterWithOptions(t, nil, nil, false)
	// 不迁移 content_items——装饰查询必然失败。

	require.NoError(t, db.Create(&[]model.User{
		{ID: 1, Email: "alice@example.test", PasswordHash: "hash", Username: "alice"},
		{ID: 2, Email: "bob@example.test", PasswordHash: "hash", Username: "bob"},
	}).Error)
	require.NoError(t, db.Create(&model.Conversation{ID: 10}).Error)
	require.NoError(t, db.Create(&[]model.ConversationParticipant{
		{ConversationID: 10, UserID: 1},
		{ConversationID: 10, UserID: 2},
	}).Error)
	seedInviteMessage(t, db, 21, "invite before content_items exists", model.JSONMap{
		"invite_id": 201, "content_id": 701, "content_title": "Original Work",
		"content_zone": "original",
		"inviter_id":   1, "inviter_username": "alice",
	}, time.Now().UTC().Add(-time.Minute))

	rec := listMessagesAs(t, router, 2, 10)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Messages []struct {
			ID       int64          `json:"id"`
			Metadata map[string]any `json:"metadata"`
		} `json:"messages"`
		Total int64 `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Messages, 1)
	require.Equal(t, "original", body.Messages[0].Metadata["content_zone"], "old metadata must survive a failed decoration")
	require.NotContains(t, body.Messages[0].Metadata, "content_url")
	require.EqualValues(t, 1, body.Total, "other list fields must not be lost")
}

// 会话参与者校验继续生效：非参与者拿不到消息（也就拿不到装饰后的导航事实）。
func TestListMessagesInviteNavigationKeepsParticipantGuard(t *testing.T) {
	router, db := setupMessageRouterWithOptions(t, nil, nil, false)
	require.NoError(t, db.AutoMigrate(&model.ContentItem{}))

	require.NoError(t, db.Create(&[]model.User{
		{ID: 1, Email: "alice@example.test", PasswordHash: "hash", Username: "alice"},
		{ID: 2, Email: "bob@example.test", PasswordHash: "hash", Username: "bob"},
		{ID: 3, Email: "eve@example.test", PasswordHash: "hash", Username: "eve"},
	}).Error)
	require.NoError(t, db.Create(&model.ContentItem{
		ID: 701, Title: "Original Work", AuthorID: 1, Zone: "original", ContentType: "article", Status: "published",
	}).Error)
	require.NoError(t, db.Create(&model.Conversation{ID: 10}).Error)
	require.NoError(t, db.Create(&[]model.ConversationParticipant{
		{ConversationID: 10, UserID: 1},
		{ConversationID: 10, UserID: 2},
	}).Error)
	seedInviteMessage(t, db, 31, "guarded invite", model.JSONMap{
		"invite_id": 301, "content_id": 701, "content_title": "Original Work",
		"inviter_id": 1, "inviter_username": "alice",
	}, time.Now().UTC().Add(-time.Minute))

	rec := listMessagesAs(t, router, 3, 10)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "FORBIDDEN")
}

func seedInviteMessage(t *testing.T, db *gorm.DB, id int64, bodyText string, metadata model.JSONMap, createdAt time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&model.Message{
		ID:             id,
		ConversationID: 10,
		SenderID:       1,
		Body:           bodyText,
		MsgType:        "collab_invite",
		Metadata:       metadata,
		CreatedAt:      createdAt,
	}).Error)
}
