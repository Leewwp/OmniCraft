package handler

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// Avatar moderation sentinel errors for the UpdateUser avatar gate. Callers
// map them to the unified {code, message} response format; raw scanner errors
// never reach the client.
var (
	ErrAvatarBlocked               = errors.New("avatar image blocked by content moderation")
	ErrAvatarModerationUnavailable = errors.New("avatar moderation unavailable")
)

// avatarReviewer is the minimal image-moderation dependency UserHandler needs
// to gate avatar_url updates before they are persisted. *service.ReviewService
// satisfies it; tests inject a fake.
type avatarReviewer interface {
	ReviewImageURL(ctx context.Context, imageURL string) (string, error)
}

type UserHandler struct {
	userRepo       *repository.UserRepository
	reputSvc       *service.ReputationService
	contentRepo    *repository.ContentRepository
	followRepo     *repository.FollowRepository
	authSvc        *service.AuthService
	rdb            *redis.Client
	cfg            *config.Config
	jwtSecret      string
	avatarReviewer avatarReviewer
	displaySigner  *service.DisplayURLSigner
}

func NewUserHandler(db *gorm.DB, authSvc *service.AuthService, rdb *redis.Client, cfg *config.Config, reviewers ...avatarReviewer) *UserHandler {
	h := &UserHandler{
		userRepo:      repository.NewUserRepository(db),
		reputSvc:      service.NewReputationService(db),
		contentRepo:   repository.NewContentRepository(db),
		followRepo:    repository.NewFollowRepository(db),
		authSvc:       authSvc,
		rdb:           rdb,
		cfg:           cfg,
		jwtSecret:     cfg.JWT.Secret,
		displaySigner: service.NewDisplayURLSigner(cfg),
	}
	if len(reviewers) > 0 {
		h.avatarReviewer = reviewers[0]
	}
	return h
}

func (h *UserHandler) GetUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	user, err := h.userRepo.FindByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
			return
		}
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	var resp gin.H
	if isSelfOrAdmin(c, id) {
		resp = h.sanitizeUser(user)
	} else {
		resp = h.publicUser(user)
	}
	stats, followersCount := h.getUserStatsAndFollowers(id)
	resp["stats"] = stats
	resp["followers_count"] = followersCount
	resp["is_following"] = h.checkIsFollowing(c, id)

	c.JSON(http.StatusOK, gin.H{"user": resp})
}

func (h *UserHandler) getUserStatsAndFollowers(userID int64) (gin.H, int64) {
	var contentsCount int64
	var likesReceived int64
	var followersCount int64

	h.contentRepo.DB().Raw(
		`SELECT
			(SELECT COUNT(*) FROM content_items WHERE author_id = ? AND status = 'published') AS contents_count,
			COALESCE((SELECT SUM(like_count) FROM content_items WHERE author_id = ? AND status = 'published'), 0) AS likes_received,
			(SELECT COUNT(*) FROM follows WHERE target_type = 'user' AND target_id = ?) AS followers_count`,
		userID, userID, userID,
	).Scan(&struct {
		Count     *int64
		Likes     *int64
		Followers *int64
	}{Count: &contentsCount, Likes: &likesReceived, Followers: &followersCount})

	return gin.H{
		"contents_count": contentsCount,
		"likes_received": likesReceived,
	}, followersCount
}

func (h *UserHandler) checkIsFollowing(c *gin.Context, targetID int64) bool {
	callerID := middleware.GetUserID(c)
	if callerID == 0 {
		return false
	}
	following, err := h.followRepo.IsFollowing(callerID, "user", targetID)
	if err != nil {
		return false
	}
	return following
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	callerID := middleware.GetUserID(c)
	if callerID != id {
		c.JSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "can only update your own profile"})
		return
	}

	var req struct {
		Username            *string `json:"username"`
		AvatarURL           *string `json:"avatar_url"`
		Bio                 *string `json:"bio"`
		PreferredLocale     *string `json:"preferred_locale"`
		AcceptCollabInvites *bool   `json:"accept_collab_invites"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.SafeErrorResponse(c, http.StatusBadRequest, "INVALID_BODY", err)
		return
	}

	updates := map[string]interface{}{}
	if req.Username != nil {
		updates["username"] = *req.Username
	}
	if req.AvatarURL != nil {
		avatarURL := strings.TrimSpace(*req.AvatarURL)
		if avatarURL != "" {
			if h.cfg == nil || !aliyun.IsPlatformObjectURL(h.cfg.OSS.Domain, avatarURL) {
				response.Error(c, http.StatusBadRequest, "AVATAR_NOT_PLATFORM_OSS_OBJECT", "avatar must be a platform OSS object URL")
				return
			}
			if err := h.reviewAvatarImage(c, avatarURL); err != nil {
				switch {
				case errors.Is(err, ErrAvatarBlocked):
					response.Error(c, http.StatusBadRequest, "AVATAR_BLOCKED", "avatar image was blocked by content moderation")
				case errors.Is(err, ErrAvatarModerationUnavailable):
					response.Error(c, http.StatusServiceUnavailable, "MODERATION_UNAVAILABLE", "avatar moderation is temporarily unavailable, please try again later")
				default:
					response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
				}
				return
			}
		}
		// Empty avatar_url clears the avatar back to the platform default and
		// skips both the platform-object gate and the image scan.
		updates["avatar_url"] = avatarURL
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.PreferredLocale != nil {
		updates["preferred_locale"] = *req.PreferredLocale
	}
	if req.AcceptCollabInvites != nil {
		updates["accept_collab_invites"] = *req.AcceptCollabInvites
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_FIELDS", "message": "no fields to update"})
		return
	}

	if err := h.userRepo.UpdateFields(id, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	user, _ := h.userRepo.FindByID(id)
	c.JSON(http.StatusOK, gin.H{"user": h.sanitizeUser(user)})
}

func (h *UserHandler) GetReputation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	logs, total, err := h.reputSvc.GetLogs(id, page, pageSize)
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *UserHandler) GetUserContents(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "invalid user id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentType := c.Query("content_type")

	items, total, err := h.contentRepo.ListContents(repository.ListContentsFilter{
		AuthorID:    &id,
		ViewerID:    middleware.GetUserID(c),
		ContentType: contentType,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateContents(items)
	c.JSON(http.StatusOK, gin.H{"contents": items, "total": total})
}

// GetMyContributors serves GET /users/me/contributors (T50/FIX-22d): the
// server-side contributor aggregation for the studio page. Per user: merged
// PR count across the caller's contents, source (merged|invite) and the real
// author-blocklist state. Replaces the page's per-content PR fan-out with a
// single request.
func (h *UserHandler) GetMyContributors(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	db := h.contentRepo.DB()

	type contributorRow struct {
		UserID   int64
		Username string
		PRCount  int
		Blocked  int
	}
	var rows []contributorRow
	if err := db.Raw(
		`SELECT cc.user_id,
		        u.username,
		        COALESCE(SUM(cc.pr_count), 0) AS pr_count,
		        MAX(CASE WHEN ab.blocked_id IS NOT NULL THEN 1 ELSE 0 END) AS blocked
		 FROM content_contributors cc
		 JOIN content_items c ON c.id = cc.content_item_id AND c.deleted_at IS NULL
		 JOIN users u ON u.id = cc.user_id
		 LEFT JOIN author_blocklist ab ON ab.author_id = ? AND ab.blocked_id = cc.user_id
		 WHERE c.author_id = ?
		 GROUP BY cc.user_id, u.username
		 ORDER BY pr_count DESC, cc.user_id ASC`, callerID, callerID).Scan(&rows).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	type contributor struct {
		UserID   int64  `json:"user_id"`
		Username string `json:"username"`
		PRCount  int    `json:"pr_count"`
		Source   string `json:"source"`
		Blocked  bool   `json:"blocked"`
	}
	contributors := make([]contributor, 0, len(rows))
	for _, row := range rows {
		// pr_count=0 rows only come from accepted collaboration invites
		// (InsertContributorIfAbsent); merged PRs always bump the count.
		source := "invite"
		if row.PRCount > 0 {
			source = "merged"
		}
		contributors = append(contributors, contributor{
			UserID:   row.UserID,
			Username: row.Username,
			PRCount:  row.PRCount,
			Source:   source,
			Blocked:  row.Blocked == 1,
		})
	}
	c.JSON(http.StatusOK, gin.H{"contributors": contributors, "total": len(contributors)})
}

// GetMyPendingTasks serves GET /users/me/pending-tasks (T49/FIX-22c): the
// caller's to-dos in one request — open PRs and pending tag suggestions on
// their own contents. Items on other users' contents never appear.
func (h *UserHandler) GetMyPendingTasks(c *gin.Context) {
	callerID := middleware.GetUserID(c)
	db := h.contentRepo.DB()

	type pendingTask struct {
		Type  string `json:"type"`
		ID    int64  `json:"id"`
		Title string `json:"title"`
	}
	tasks := make([]pendingTask, 0)

	var prs []struct {
		ID      int64
		Message string
		Title   string
	}
	if err := db.Raw(
		`SELECT pr.id, pr.message, c.title AS title
		 FROM pull_requests pr JOIN content_items c ON c.id = pr.content_item_id
		 WHERE c.author_id = ? AND pr.status = 'open' AND c.deleted_at IS NULL
		 ORDER BY pr.id DESC LIMIT 50`, callerID).Scan(&prs).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	for _, pr := range prs {
		title := pr.Title
		if pr.Message != "" {
			title = pr.Message
		}
		tasks = append(tasks, pendingTask{Type: "pr", ID: pr.ID, Title: title})
	}

	var sugs []struct {
		ID     int64
		Tag    string
		Action string
		Title  string
	}
	if err := db.Raw(
		`SELECT ts.id, ts.tag, ts.action, c.title AS title
		 FROM tag_suggestions ts JOIN content_items c ON c.id = ts.content_item_id
		 WHERE c.author_id = ? AND ts.status = 'pending' AND c.deleted_at IS NULL
		 ORDER BY ts.id DESC LIMIT 50`, callerID).Scan(&sugs).Error; err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	for _, sug := range sugs {
		verb := sug.Action
		if verb == "add" {
			verb = "添加"
		} else if verb == "remove" {
			verb = "移除"
		}
		tasks = append(tasks, pendingTask{Type: "tag", ID: sug.ID, Title: "《" + sug.Title + "》标签" + verb + "：" + sug.Tag})
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": len(tasks)})
}

func (h *UserHandler) GetMyContents(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	contentType := c.Query("content_type")

	items, total, err := h.contentRepo.ListContents(repository.ListContentsFilter{
		AuthorID:    &callerID,
		ContentType: contentType,
		Page:        page,
		PageSize:    pageSize,
		// 作者自助列表包含全部状态（banned 行需携带 ban_reason 供作者知晓）。
		IncludeAllStatuses: true,
	})
	if err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}
	h.displaySigner.DecorateContents(items)
	// 作者自助路径：附加 ban_reason（model 层不序列化，FIX-16）。
	payloads := make([]map[string]any, 0, len(items))
	for i := range items {
		m, err := contentWithBanReason(&items[i])
		if err != nil {
			response.SafeErrorResponse(c, http.StatusInternalServerError, "INTERNAL_ERROR", err)
			return
		}
		payloads = append(payloads, m)
	}
	c.JSON(http.StatusOK, gin.H{"contents": payloads, "total": total})
}

func (h *UserHandler) ChangePassword(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	user, err := h.userRepo.FindByID(callerID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "INVALID_PASSWORD", "message": "old password is incorrect"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL_ERROR", "message": "failed to hash password"})
		return
	}

	if err := h.userRepo.UpdateFields(callerID, map[string]interface{}{"password_hash": string(hash)}); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	invalidateUserTokens(h.rdb, callerID)

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

func (h *UserHandler) DeleteAccount(c *gin.Context) {
	callerID := middleware.GetUserID(c)

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	user, err := h.userRepo.FindByID(callerID)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "USER_NOT_FOUND", "message": "user not found"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "INVALID_PASSWORD", "message": "password is incorrect"})
		return
	}

	anonName := fmt.Sprintf("已注销用户_%d", callerID)
	anonEmail := fmt.Sprintf("deleted_%d@anon.local", callerID)
	randomHash, _ := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(make([]byte, 32))), bcrypt.DefaultCost)
	// T30（FIX-20）：注销 = deleted_at 软删除 + 匿名化清写（username/email/
	// avatar/bio，防 PII 残留）；不再伪装封禁（is_banned 不设、ban_reason 清空），
	// 鉴权走 middleware 现成 deleted → 401 "user not found or deleted" 分支。
	deletedAt := time.Now()
	updates := map[string]interface{}{
		"username":      anonName,
		"email":         anonEmail,
		"avatar_url":    "",
		"bio":           "",
		"deleted_at":    deletedAt,
		"ban_reason":    "",
		"password_hash": string(randomHash),
	}

	if err := h.userRepo.UpdateFields(callerID, updates); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	invalidateUserTokens(h.rdb, callerID)
	middleware.InvalidateUserStatusCache(h.rdb, callerID)

	c.JSON(http.StatusOK, gin.H{"message": "account deleted successfully"})
}

func (h *UserHandler) UpdateSupportInfo(c *gin.Context) {
	if !h.cfg.Features.CreatorSupportEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "FEATURE_DISABLED", "message": "creator support is not enabled"})
		return
	}

	callerID := middleware.GetUserID(c)

	var req struct {
		DonationImageURL string   `json:"donation_image_url"`
		ExternalLinks    []string `json:"external_links"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "invalid request parameters")
		return
	}

	if len(req.ExternalLinks) > 3 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "VALIDATION_ERROR", "message": "external_links maximum is 3"})
		return
	}

	info := model.JSONMap{
		"donation_image_url": req.DonationImageURL,
		"external_links":     req.ExternalLinks,
	}

	if err := h.userRepo.UpdateFields(callerID, map[string]interface{}{"support_info": info}); err != nil {
		response.SafeErrorResponse(c, http.StatusInternalServerError, "DB_ERROR", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "support info updated", "support_info": info})
}

func invalidateUserTokens(rdb *redis.Client, userID int64) {
	if rdb == nil {
		return
	}
	ctx := context.Background()
	tokenSetKey := fmt.Sprintf("user:tokens:%d", userID)
	members, err := rdb.SMembers(ctx, tokenSetKey).Result()
	if err != nil {
		return
	}
	if len(members) > 0 {
		rdb.Del(ctx, members...)
	}
	rdb.Del(ctx, tokenSetKey)
}

// reviewAvatarImage runs the avatar image moderation gate before a new
// avatar_url is persisted. The availability policy follows the A4 environment
// semantics shared by every moderation gate via service.RunModerationGate: in
// release mode any moderation failure is fail-closed, while in local/test
// mode an unconfigured Green client is fail-open and must be recorded via
// structured logs.
func (h *UserHandler) reviewAvatarImage(c *gin.Context, avatarURL string) error {
	var review func(context.Context) (string, error)
	if h.avatarReviewer != nil {
		review = func(ctx context.Context) (string, error) {
			return h.avatarReviewer.ReviewImageURL(ctx, avatarURL)
		}
	}
	return service.RunModerationGate(c.Request.Context(), h.cfg, "avatar_review", "avatar moderation", "update",
		review, true, ErrAvatarBlocked, ErrAvatarModerationUnavailable)
}

// sanitizeUser projects a user for the SELF view (profile update response):
// email and preferred_locale are included because the caller is the user.
// The avatar is a private-OSS display URL, so it crosses the boundary signed
// (B-002); the caller's model value is left untouched.
func (h *UserHandler) sanitizeUser(u *model.User) gin.H {
	resp := h.publicUser(u)
	resp["email"] = u.Email
	resp["preferred_locale"] = u.PreferredLocale
	return resp
}

// publicUser projects a user for anonymous/other viewers: no email, no
// preferred_locale (FIX-19a — registration email must never leak to others).
func (h *UserHandler) publicUser(u *model.User) gin.H {
	avatarURL := u.AvatarURL
	if h.displaySigner != nil {
		avatarURL = h.displaySigner.SignURL(avatarURL)
	}
	return gin.H{
		"id":                    u.ID,
		"username":              u.Username,
		"avatar_url":            avatarURL,
		"bio":                   u.Bio,
		"reputation":            u.Reputation,
		"role":                  u.Role,
		"is_banned":             u.IsBanned,
		"accept_collab_invites": u.AcceptCollabInvites,
		"created_at":            u.CreatedAt,
	}
}

// isSelfOrAdmin reports whether the caller may see the self-tier projection
// (email, preferred_locale) of the target user.
func isSelfOrAdmin(c *gin.Context, targetID int64) bool {
	if middleware.GetUserID(c) == targetID {
		return true
	}
	role, exists := c.Get(middleware.UserRoleKey)
	return exists && role == "admin"
}
