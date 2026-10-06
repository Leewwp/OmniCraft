package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

// run-1 审计 #6 生命周期清理：IP approve/reject 状态转换时关闭其开放提案
// ——隐藏（rejected）IP 不保留可投票的共治面；approve 转换同样收口 pending
// 期提案（re-approve 幂等：已 approved 的 IP 再次 approve 不得误杀在途提案）。

func setupIPLifecycleRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.IPTag{}, &model.AdminAuditLog{},
		&model.AIReviewRecord{}, &model.IPReviewLog{}, &model.Notification{},
		&model.IPProposal{}, &model.IPProposalVote{}, &model.IPProfileVersion{},
	))

	cfg := &config.Config{}
	auditSvc := service.NewAdminAuditService(repository.NewAdminAuditRepository(db), db)
	adminHandler := newAdminHandlerForTest(db, cfg, nil, auditSvc)
	adminHandler.SetNotificationService(service.NewNotificationService(repository.NewNotificationRepository(db)))

	router := gin.New()
	register := func(method, path string, h gin.HandlerFunc) {
		router.Handle(method, path, func(c *gin.Context) {
			c.Set(middleware.UserIDKey, int64(1))
			c.Set("trace_id", "trace-ip-lifecycle-test")
			h(c)
		})
	}
	register(http.MethodPost, "/admin/ips/:id/reject", adminHandler.RejectIP)
	register(http.MethodPost, "/admin/ips/:id/approve", adminHandler.ApproveIP)
	return router, db
}

func seedLifecycleIPWithOpenProposal(t *testing.T, db *gorm.DB, status string) (ipID, proposalID int64) {
	t.Helper()
	admin := model.User{ID: 1, Email: "lc-admin@example.test", Username: "lc-admin", PasswordHash: "hash", Role: "admin", Reputation: 10}
	require.NoError(t, db.Create(&admin).Error)
	creator := model.User{ID: 2, Email: "lc-creator@example.test", Username: "lc-creator", PasswordHash: "hash", Role: "user", Reputation: 10}
	require.NoError(t, db.Create(&creator).Error)
	ip := model.IP{Name: "LC IP", Slug: "lc-ip", Category: "game", CreatorID: &creator.ID, Status: status}
	require.NoError(t, db.Create(&ip).Error)
	desc := "生命周期测试提案"
	proposal := model.IPProposal{
		IPID: ip.ID, ProposerID: creator.ID, Status: "open",
		DescriptionChange: &desc, ModerationState: "approved",
		DeadlineAt: time.Now().Add(48 * time.Hour),
	}
	require.NoError(t, db.Create(&proposal).Error)
	return ip.ID, proposal.ID
}

func TestRejectIP_ClosesOpenProposals(t *testing.T) {
	router, db := setupIPLifecycleRouter(t)
	ipID, proposalID := seedLifecycleIPWithOpenProposal(t, db, "pending")

	req := httptest.NewRequest(http.MethodPost, "/admin/ips/"+itoa64(ipID)+"/reject", strings.NewReader(`{"reason":"内容不合规"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var proposal model.IPProposal
	require.NoError(t, db.First(&proposal, proposalID).Error)
	require.Equal(t, "rejected", proposal.Status, "rejecting an ip must close its open proposals")
	require.NotNil(t, proposal.ClosedAt)
}

func TestApproveIP_TransitionClosesProposalsButReApproveDoesNot(t *testing.T) {
	router, db := setupIPLifecycleRouter(t)
	ipID, firstID := seedLifecycleIPWithOpenProposal(t, db, "pending")

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/admin/ips/"+itoa64(ipID)+"/approve", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	rec := post()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var first model.IPProposal
	require.NoError(t, db.First(&first, firstID).Error)
	require.Equal(t, "rejected", first.Status, "the pending→approved transition must close proposals created while hidden")

	// approve 后的公开共治提案在重复 approve（幂等重放）时必须存活。
	publicDesc := "approve 后的公开提案"
	second := model.IPProposal{
		IPID: ipID, ProposerID: 2, Status: "open",
		DescriptionChange: &publicDesc, ModerationState: "approved",
		DeadlineAt: time.Now().Add(48 * time.Hour),
	}
	require.NoError(t, db.Create(&second).Error)
	rec = post()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var reloaded model.IPProposal
	require.NoError(t, db.First(&reloaded, second.ID).Error)
	require.Equal(t, "open", reloaded.Status, "re-approving an already-approved ip must not kill live public proposals")
}
