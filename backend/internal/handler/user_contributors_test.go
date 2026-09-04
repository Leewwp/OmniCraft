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

// T50 (FIX-22d): GET /users/me/contributors — server-side contributor
// aggregation for the studio page: per-user merged PR count, source
// (merged|invite), and real blocklist state. Front-end assembly (previously
// N+1 per-content PR calls with blocked hard-wired to false) is replaced.

func setupT50ContributorsRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.ContentItem{}, &model.ContentContributor{},
		&model.AuthorBlocklist{}, &model.PullRequest{},
	))
	require.NoError(t, db.Exec(`CREATE TABLE IF NOT EXISTS collaboration_invites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		content_id BIGINT NOT NULL,
		inviter_id BIGINT NOT NULL,
		invitee_id BIGINT NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error)

	userHandler := NewUserHandler(db, nil, redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"}), &config.Config{})

	router := gin.New()
	router.Handle(http.MethodGet, "/users/me/contributors", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, int64(1))
		userHandler.GetMyContributors(c)
	})
	return router, db
}

func TestGetMyContributorsAggregatesMergedInviteAndBlocked(t *testing.T) {
	router, db := setupT50ContributorsRouter(t)

	owner := model.User{ID: 1, Email: "t50-owner@example.test", Username: "t50-owner", PasswordHash: "hash", Reputation: 10}
	merged := model.User{ID: 2, Email: "t50-merged@example.test", Username: "t50-merged", PasswordHash: "hash", Reputation: 10}
	invited := model.User{ID: 3, Email: "t50-invited@example.test", Username: "t50-invited", PasswordHash: "hash", Reputation: 10}
	blocked := model.User{ID: 4, Email: "t50-blocked@example.test", Username: "t50-blocked", PasswordHash: "hash", Reputation: 10}
	for _, u := range []model.User{owner, merged, invited, blocked} {
		require.NoError(t, db.Create(&u).Error)
	}

	mine := model.ContentItem{ID: 10, Title: "我的内容", AuthorID: owner.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	mine2 := model.ContentItem{ID: 30, Title: "我的另一篇", AuthorID: owner.ID, Zone: "fanwork", ContentType: "article", Status: "published"}
	foreign := model.ContentItem{ID: 20, Title: "别人的内容", AuthorID: 3, Zone: "fanwork", ContentType: "article", Status: "published"}
	require.NoError(t, db.Create(&mine).Error)
	require.NoError(t, db.Create(&mine2).Error)
	require.NoError(t, db.Create(&foreign).Error)

	// merged 贡献者：在我两条内容上共合并 3 个 PR
	require.NoError(t, db.Exec(`INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at) VALUES (10, 2, 2, CURRENT_TIMESTAMP), (30, 2, 1, CURRENT_TIMESTAMP)`).Error)
	// invite 贡献者：pr_count=0，由协作邀请产生
	require.NoError(t, db.Exec(`INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at) VALUES (10, 3, 0, CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO collaboration_invites (id, content_id, inviter_id, invitee_id, status) VALUES (1, 10, 1, 3, 'accepted')`).Error)
	// 被屏蔽的贡献者：有合并贡献且在 blocklist
	require.NoError(t, db.Exec(`INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at) VALUES (10, 4, 1, CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, db.Create(&model.AuthorBlocklist{AuthorID: owner.ID, BlockedID: blocked.ID}).Error)
	// 不应出现：别人内容上的贡献者
	require.NoError(t, db.Exec(`INSERT INTO content_contributors (content_item_id, user_id, pr_count, first_at) VALUES (20, 4, 5, CURRENT_TIMESTAMP)`).Error)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/me/contributors", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Contributors []struct {
			UserID            int64  `json:"user_id"`
			Username          string `json:"username"`
			PRCount           int    `json:"pr_count"`
			Source            string `json:"source"`
			Blocked           bool   `json:"blocked"`
		} `json:"contributors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Contributors, 3, "3 位贡献者（合并/邀请/被屏蔽），别人内容的贡献者不出现")

	byUser := map[int64]struct {
		PRCount int
		Source  string
		Blocked bool
	}{}
	for _, c := range resp.Contributors {
		byUser[c.UserID] = struct {
			PRCount int
			Source  string
			Blocked bool
		}{c.PRCount, c.Source, c.Blocked}
	}

	require.Equal(t, 3, byUser[2].PRCount, "merged 贡献者跨内容聚合 PR 计数")
	require.Equal(t, "merged", byUser[2].Source)
	require.False(t, byUser[2].Blocked)

	require.Equal(t, "invite", byUser[3].Source, "pr_count=0 且有 accepted 邀请 = invite 来源")
	require.False(t, byUser[3].Blocked)

	require.Equal(t, "merged", byUser[4].Source)
	require.True(t, byUser[4].Blocked, "blocked 必须来自 author_blocklist 真实状态")
}
