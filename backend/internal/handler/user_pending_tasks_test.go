package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

// T49 (FIX-22c): GET /users/me/pending-tasks — one request returning the
// owner's pending tag suggestions and open PRs on their own contents. Items
// belonging to other users' contents must never appear.

func setupT49PendingRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.ContentItem{}, &model.TagSuggestion{}, &model.PullRequest{}))

	userHandler := NewUserHandler(db, nil, redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), &config.Config{})

	router := gin.New()
	router.Handle(http.MethodGet, "/users/me/pending-tasks", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		userHandler.GetMyPendingTasks(c)
	})
	return router, db
}

func TestGetMyPendingTasksAggregatesTagSuggestionsAndPRs(t *testing.T) {
	router, db := setupT49PendingRouter(t)

	owner := model.User{ID: 1, Email: "t49-owner@example.test", Username: "t49-owner", PasswordHash: "hash", Reputation: 10}
	other := model.User{ID: 2, Email: "t49-other@example.test", Username: "t49-other", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&owner).Error)
	require.NoError(t, db.Create(&other).Error)

	mine := model.ContentItem{ID: 10, Title: "我的内容", AuthorID: owner.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	foreign := model.ContentItem{ID: 20, Title: "别人的内容", AuthorID: other.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&mine).Error)
	require.NoError(t, db.Create(&foreign).Error)

	// 我的待办：1 条 pending 标签建议 + 1 条 open PR
	require.NoError(t, db.Create(&model.TagSuggestion{ID: 100, ContentItemID: mine.ID, UserID: other.ID, Tag: "奇幻", Action: "add", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.PullRequest{ID: 200, ContentItemID: mine.ID, SubmitterID: other.ID, BaseVersionID: 1, Status: "open", Message: "修正错别字"}).Error)
	// 不应出现：别人的内容上的建议 + 我已接受的 PR
	require.NoError(t, db.Create(&model.TagSuggestion{ID: 101, ContentItemID: foreign.ID, UserID: owner.ID, Tag: "科幻", Action: "add", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.PullRequest{ID: 201, ContentItemID: mine.ID, SubmitterID: other.ID, BaseVersionID: 1, Status: "accepted", Message: "已接受的旧 PR"}).Error)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/me/pending-tasks", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Tasks []struct {
			Type  string `json:"type"`
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"tasks"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, 2, resp.Total, "仅聚合本人内容的 open PR + pending 标签建议")

	byType := map[string]int64{}
	for _, task := range resp.Tasks {
		byType[task.Type] = task.ID
		switch task.Type {
		case "pr":
			require.EqualValues(t, 200, task.ID)
			require.Contains(t, task.Title, "修正错别字", "PR 条目应携带 message")
		case "tag":
			require.EqualValues(t, 100, task.ID)
			require.Contains(t, task.Title, "奇幻", "标签建议条目应携带 tag")
		}
	}
	require.EqualValues(t, 200, byType["pr"])
	require.EqualValues(t, 100, byType["tag"])
}
