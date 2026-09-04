package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/handler"
	"omnicraft/backend/internal/middleware"
)

func RegisterRoutes(v1 *gin.RouterGroup, cfg *config.Config, ctr *container.ServiceContainer) {
	rdb := ctr.RDB
	db := ctr.DB
	userRepo := ctr.UserRepo
	authService := ctr.AuthService
	// displaySigner issues short-lived signed GET URLs for display media
	// (covers, avatars, gallery attachments) at the API serialization
	// boundary; nil-safe passthrough when OSS is not configured (B-002).
	displaySigner := ctr.DisplayURLSigner
	authHandler := handler.NewAuthHandler(authService, ctr.VerificationService, userRepo, ctr.CaptchaVerifier, rdb, cfg)

	notifSvc := ctr.NotificationService

	optAuth := middleware.OptionalAuth(cfg, rdb, db)
	authReq := middleware.AuthRequired(cfg, rdb, db)
	searchLimiter := middleware.RedisFixedWindowLimit(
		rdb,
		"ratelimit:search",
		cfg.RateLimit.SearchPerMinute,
		time.Minute,
		false,
	)

	publishGuard := middleware.InteractionRequired(cfg, db, rdb, publishingInteractionPolicy())
	editDeleteGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	commentsGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	reactionsGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	collectionGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	seriesGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	reportsGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	prGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	judgeGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	followsGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	messagesGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	downloadsGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	agentGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())
	collabInvitesGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())

	publicConfigHandler := handler.NewPublicConfigHandler(cfg)
	v1.GET("/config/public", publicConfigHandler.GetPublicConfig)
	captchaHandler := handler.NewCaptchaHandler(ctr.CaptchaProvider, ctr.CaptchaTickets)
	v1.POST("/captcha/verify", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), captchaHandler.Verify)

	auth := v1.Group("/auth")
	{
		auth.POST("/register", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), authHandler.Register)
		auth.POST("/login", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/refresh", authHandler.Refresh)
		auth.GET("/me", authReq, authHandler.Me)
		auth.GET("/csrf", authHandler.CSRFToken)
		auth.POST("/verify-email", authHandler.VerifyEmail)
		auth.POST("/resend-verification", authHandler.ResendVerification)
	}
	auth.POST("/forgot-password", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), authHandler.ForgotPassword)
	auth.POST("/reset-password", authHandler.ResetPassword)

	userHandler := handler.NewUserHandler(db, authService, rdb, cfg, ctr.ReviewService)
	users := v1.Group("/users")
	{
		users.GET("/:id", optAuth, userHandler.GetUser)
		users.PATCH("/:id", authReq, userHandler.UpdateUser)
		users.GET("/:id/reputation", optAuth, userHandler.GetReputation)
		users.GET("/:id/contents", optAuth, userHandler.GetUserContents)
		users.DELETE("/me", authReq, userHandler.DeleteAccount)
		users.PATCH("/me/password", authReq, userHandler.ChangePassword)
		users.PATCH("/me/support-info", authReq, userHandler.UpdateSupportInfo)
	}

	ipHandler := handler.NewIPHandlerWithCache(db, rdb, cfg)
	ips := v1.Group("/ips")
	{
		ips.GET("", optAuth, ipHandler.ListIPs)
		// T15 (F-103): IP creation enters the review queue and publishes
		// public free text, so it carries the same publishing guard + upload
		// rate limit as content creation.
		ips.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), ipHandler.CreateIP)
		ips.GET("/:id", optAuth, ipHandler.GetIP)
		ips.GET("/:id/contents", optAuth, ipHandler.GetIPContents)

		proposalHandler := handler.NewIPProposalHandler(ctr.IPProposalService)
		ips.GET("/:id/proposals", optAuth, proposalHandler.ListProposals)
		ips.POST("/:id/proposals", authReq, proposalHandler.CreateProposal)
		ips.GET("/:id/proposals/:proposalId", optAuth, proposalHandler.GetProposal)
		ips.POST("/:id/proposals/:proposalId/vote", authReq, proposalHandler.SubmitVote)
		ips.GET("/:id/versions", optAuth, proposalHandler.ListVersions)
	}

	prHandler := handler.NewPRHandlerWithService(ctr.PRService)
	contentHandler := handler.NewContentHandler(db, cfg, rdb)
	contentHandler.SetQueueProducer(ctr.QueueProducer)
	contentHandler.SetOutboxRepository(ctr.OutboxRepo)
	contentHandler.SetArchiveScanRepository(ctr.ArchiveScanRepo)
	contents := v1.Group("/contents")
	{
		contents.GET("", optAuth, contentHandler.ListContents)
		contents.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)
		contents.POST("/oss-token", authReq, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.GenerateOSSToken)
		contents.GET("/:id/related-fanworks", optAuth, contentHandler.ListRelatedFanworks)
		contents.GET("/:id", optAuth, contentHandler.GetContent)
		contents.PATCH("/:id", authReq, editDeleteGuard, contentHandler.UpdateContent)
		contents.DELETE("/:id", authReq, editDeleteGuard, contentHandler.DeleteContent)
		contents.GET("/:id/versions", optAuth, handler.NewVersionHandler(db).ListVersions)
		contents.GET("/:id/prs", optAuth, prHandler.ListPRs)
		contents.GET("/:id/download", authReq, downloadsGuard, contentHandler.DownloadContent)
	}

	versionHandler := handler.NewVersionHandler(db)
	versions := v1.Group("/versions")
	{
		versions.GET("/:id", optAuth, versionHandler.GetVersion)
	}

	pr := v1.Group("/pr")
	{
		pr.POST("", authReq, prGuard, prHandler.SubmitPR)
		pr.GET("/:id", optAuth, prHandler.GetPR)
		pr.POST("/:id/accept", authReq, prGuard, prHandler.AcceptPR)
		pr.POST("/:id/reject", authReq, prGuard, prHandler.RejectPR)
		pr.POST("/:id/merge", authReq, prGuard, prHandler.ManualMerge)
	}

	dashboard := v1.Group("/dashboard", authReq)
	{
		dashboard.POST("/contributors/:userId/block", prHandler.BlockContributor)
		dashboard.DELETE("/contributors/:userId/block", prHandler.UnblockContributor)
	}

	socialSvc := ctr.SocialService
	socialHandler := handler.NewSocialHandlerWithService(socialSvc, db)
	socialHandler.SetDisplayURLSigner(displaySigner)
	social := v1.Group("/social")
	{
		social.GET("/comments", optAuth, socialHandler.ListComments)
		social.POST("/comments", authReq, commentsGuard, socialHandler.PostComment)
		social.DELETE("/comments/:id", authReq, commentsGuard, socialHandler.DeleteComment)
		social.PATCH("/comments/:id", authReq, commentsGuard, middleware.CommentEditRateLimit(rdb), socialHandler.EditComment)
		social.GET("/discussions", optAuth, socialHandler.ListDiscussions)
		social.POST("/discussions", authReq, socialHandler.PostDiscussion)
		social.GET("/discussions/:id", optAuth, socialHandler.GetDiscussion)
		social.POST("/reactions", authReq, reactionsGuard, socialHandler.React)
		social.GET("/reactions", optAuth, socialHandler.ListReactions)
		social.POST("/comments/:id/report", authReq, reportsGuard, socialHandler.ReportComment)
		social.GET("/reports/me", authReq, socialHandler.ListMyReports)
	}
	contents.POST("/:id/report", authReq, reportsGuard, socialHandler.ReportContent)

	collabInviteHandler := handler.NewCollabInviteHandler(ctr.CollabInviteService)
	contents.POST("/:id/collab-invites", authReq, collabInvitesGuard, collabInviteHandler.SendInvite)
	v1.POST("/collab-invites/:id/accept", authReq, collabInviteHandler.AcceptInvite)
	v1.POST("/collab-invites/:id/decline", authReq, collabInviteHandler.DeclineInvite)

	collectionHandler := handler.NewCollectionHandler(db)
	collectionHandler.SetDisplayURLSigner(displaySigner)
	v1.GET("/collections", optAuth, collectionHandler.ListCollections)
	v1.GET("/collections/:id", optAuth, collectionHandler.GetCollection)
	v1.POST("/collections", authReq, collectionGuard, collectionHandler.CreateCollection)
	v1.PUT("/collections/:id", authReq, collectionGuard, collectionHandler.UpdateCollection)
	v1.DELETE("/collections/:id", authReq, collectionGuard, collectionHandler.DeleteCollection)
	v1.POST("/collections/:id/items", authReq, collectionGuard, collectionHandler.AddItem)
	v1.DELETE("/collections/:id/items/:itemId", authReq, collectionGuard, collectionHandler.RemoveItem)
	v1.PUT("/collections/:id/items/:itemId", authReq, collectionGuard, collectionHandler.UpdateItem)

	seriesHandler := handler.NewSeriesHandler(db)
	seriesHandler.SetDisplayURLSigner(displaySigner)
	v1.POST("/series", authReq, seriesGuard, seriesHandler.CreateSeries)
	v1.GET("/series", authReq, seriesHandler.ListSeries)
	v1.GET("/series/candidates", authReq, seriesHandler.ListCandidates)
	v1.GET("/series/:id", optAuth, seriesHandler.GetSeries)
	v1.PUT("/series/:id", authReq, seriesGuard, seriesHandler.UpdateSeries)
	v1.DELETE("/series/:id", authReq, seriesGuard, seriesHandler.DeleteSeries)
	v1.POST("/series/:id/items", authReq, seriesGuard, seriesHandler.AddItem)
	v1.DELETE("/series/:id/items/:itemId", authReq, seriesGuard, seriesHandler.RemoveItem)
	v1.PUT("/series/:id/items/reorder", authReq, seriesGuard, seriesHandler.ReorderItems)

	judgeHandler := handler.NewJudgeHandler(db, cfg, ctr.AdminAuditService)
	judge := v1.Group("/judge")
	{
		judge.GET("/exam/:category", optAuth, judgeHandler.GetExam)
		judge.POST("/exam/submit", authReq, judgeGuard, judgeHandler.SubmitExam)
		judge.GET("/queue", authReq, judgeHandler.GetQueue)
		judge.POST("/vote", authReq, judgeGuard, judgeHandler.SubmitVote)
		judge.GET("/cases/:id/verdict", optAuth, judgeHandler.GetVerdictDetail)
		judge.POST("/reasons/:id/vote", authReq, judgeGuard, judgeHandler.VoteReason)
	}

	statsHandler := handler.NewStatsHandler(ctr.StatsService)
	v1.GET("/stats/summary", optAuth, statsHandler.GetSummary)

	ipStatsHandler := handler.NewIPStatsHandler(ctr.IPStatsService)
	v1.GET("/ips/stats/category_counts", optAuth, ipStatsHandler.GetCategoryCounts)

	catHandler := handler.NewCategoryHandler(db, ctr.AdminAuditService)
	v1.GET("/categories", optAuth, catHandler.ListCategories)

	tagHandler := handler.NewTagHandler(db, rdb, &cfg.Cache, cfg.RateLimit.MaxQueryChars)
	tagHandler.SetNotificationService(notifSvc)
	v1.GET("/tags/faceted", optAuth, tagHandler.GetFacetedTags)
	v1.GET("/tags/search", optAuth, tagHandler.SearchTags)
	contents.POST("/:id/tags/suggest", authReq, tagHandler.SuggestTag)
	dashboard.GET("/tag-suggestions", tagHandler.ListTagSuggestions)
	dashboard.PATCH("/tag-suggestions/:id", tagHandler.UpdateTagSuggestion)

	followHandler := handler.NewFollowHandler(db)
	followHandler.SetDisplayURLSigner(displaySigner)
	followHandler.SetNotificationService(notifSvc)

	me := v1.Group("/users/me", authReq)
	{
		me.GET("/tag-groups", tagHandler.ListTagGroups)
		me.POST("/tag-groups", tagHandler.CreateTagGroup)
		me.PATCH("/tag-groups/:id", tagHandler.UpdateTagGroup)
		me.DELETE("/tag-groups/:id", tagHandler.DeleteTagGroup)
		me.GET("/saved-searches", tagHandler.ListSavedSearches)
		me.POST("/saved-searches", tagHandler.CreateSavedSearch)
		me.DELETE("/saved-searches/:id", tagHandler.DeleteSavedSearch)
		me.GET("/followers/stats", followHandler.GetFollowerStats)
		me.GET("/contents", userHandler.GetMyContents)
		// T49: one-request to-do aggregation (open PRs + pending tag suggestions).
		me.GET("/pending-tasks", userHandler.GetMyPendingTasks)
		// T52: the creator's own IPs across every status + latest reject reason.
		me.GET("/ips", ipHandler.GetMyIPs)
	}

	searchHandler := handler.NewSearchHandler(ctr.SearchService, cfg)
	v1.GET("/search/suggestions", optAuth, searchLimiter, searchHandler.Suggestions)
	v1.GET("/search/trending", optAuth, searchLimiter, searchHandler.Trending)
	v1.GET("/contents/search", optAuth, searchLimiter, searchHandler.SearchContents)

	users.POST("/:id/follow", authReq, followsGuard, followHandler.FollowUser)
	users.DELETE("/:id/follow", authReq, followsGuard, followHandler.UnfollowUser)
	users.GET("/:id/followers", optAuth, followHandler.GetFollowers)
	users.GET("/:id/following", optAuth, followHandler.GetFollowing)
	users.GET("/search", optAuth, searchLimiter, searchHandler.SearchUsers)
	ips.POST("/:id/follow", authReq, followsGuard, followHandler.FollowIP)
	ips.DELETE("/:id/follow", authReq, followsGuard, followHandler.UnfollowIP)

	feedbackHandler := handler.NewFeedbackHandler(ctr.FeedbackService)
	feedback := v1.Group("/feedback")
	{
		feedback.POST("", optAuth, feedbackHandler.SubmitTicket)
		feedback.POST("/attachments/presign", optAuth, feedbackHandler.PresignUpload)
		feedback.GET("/me", authReq, feedbackHandler.ListMyTickets)
		feedback.GET("/:id", authReq, feedbackHandler.GetTicket)
	}

	appealHandler := handler.NewAppealHandler(db)
	v1.POST("/appeals", authReq, appealHandler.SubmitAppeal)
	v1.GET("/appeals/me", authReq, appealHandler.GetMyAppeals)

	notifHandler := handler.NewNotificationHandler(db)
	notif := v1.Group("/notifications", authReq)
	{
		notif.GET("", notifHandler.ListNotifications)
		notif.PATCH("/:id/read", notifHandler.MarkRead)
		notif.POST("/read-all", notifHandler.MarkAllRead)
		notif.GET("/unread-count", notifHandler.UnreadCount)
	}

	msgHandler := handler.NewMessageHandler(db)
	msgHandler.SetNotificationService(notifSvc)
	msgHandler.SetReviewService(cfg, ctr.ReviewService)
	messages := v1.Group("/messages", authReq)
	{
		messages.GET("", messagesGuard, msgHandler.ListConversations)
		messages.POST("", messagesGuard, msgHandler.SendMessage)
		messages.GET("/:id", msgHandler.ListMessages)
		messages.DELETE("/:id", msgHandler.DeleteMessage)
		messages.DELETE("/conversations/:id", msgHandler.LeaveConversation)
	}

	histHandler := handler.NewBrowseHistoryHandler(db, cfg)
	me.POST("/history", histHandler.RecordView)
	me.GET("/history", histHandler.GetHistory)
	me.DELETE("/history", histHandler.ClearHistory)

	ipVisitHistoryHandler := handler.NewIPVisitHistoryHandler(db)
	ipVisitHistoryHandler.SetDisplayURLSigner(displaySigner)
	me.GET("/ip-visits", ipVisitHistoryHandler.ListRecent)
	me.PUT("/ip-visits/:ipId", ipVisitHistoryHandler.RecordVisit)
	me.POST("/ip-visits/merge", ipVisitHistoryHandler.MergeVisits)

	// T12（FIX-18）：注入共享 SocialService——讨论发帖/回复统一走信誉门 +
	// Green 审核 + 楼主通知，与 /social 路由同一套治理。
	discHandler := handler.NewDiscussionHandler(db, socialSvc)
	discHandler.SetDisplayURLSigner(displaySigner)
	discHandler.SetConfig(cfg)
	ips.GET("/:id/discussions", optAuth, discHandler.ListDiscussions)
	ips.POST("/:id/discussions", authReq, discHandler.CreateDiscussion)
	ips.GET("/:id/discussions/search", optAuth, discHandler.SearchDiscussions)
	users.GET("/:id/discussions", optAuth, discHandler.ListByUser)
	discussions := v1.Group("/discussions")
	{
		discussions.GET("/:id", optAuth, discHandler.GetDiscussion)
		discussions.POST("/:id/comments", authReq, commentsGuard, discHandler.ReplyToDiscussion)
		// 置顶权收归系统管理员（#290 三轮裁决）；前端暂无入口，仅 API 操作
		discussions.PATCH("/:id/pin", authReq, middleware.AdminRequired(), discHandler.PinDiscussion)
	}

	repHandler := handler.NewReputationHandler(db)
	v1.GET("/reputation-logs/me", authReq, repHandler.GetMyReputationLogs)

	agentHandler := handler.NewAgentHandlerWithService(db, cfg, rdb, ctr.AgentService)
	agentHandler.SetQueueProducer(ctr.QueueProducer)
	// Quota for Provider-consuming routes is reserved inside each handler
	// right before the first Provider call (feature/schema/visibility checks
	// precede it and never consume quota). Conversation history and deletion
	// routes are read/write-history only and stay outside any quota path.
	agent := v1.Group("/agent", authReq, agentGuard)
	{
		agent.POST("/upload-assist", agentHandler.UploadAssist)
		agent.POST("/compliance-check", agentHandler.ComplianceCheck)
		agent.GET("/usage-guide/:id", agentHandler.UsageGuide)
		agent.POST("/chat/stream", agentHandler.ChatStream)
		agent.GET("/conversations", agentHandler.ListConversations)
		agent.GET("/conversations/:id", agentHandler.GetConversationMessages)
		agent.PATCH("/conversations/:id", agentHandler.UpdateConversation)
		agent.DELETE("/conversations/:id", agentHandler.DeleteConversation)
	}

	rehabHandler := handler.NewRehabHandler(db, rdb, cfg)
	rehab := v1.Group("/rehab", authReq)
	{
		rehab.GET("/courses", rehabHandler.ListCourses)
		rehab.GET("/courses/:id", rehabHandler.GetCourse)
		rehab.POST("/courses/:id/start", rehabHandler.StartCourse)
		rehab.POST("/courses/:id/complete", rehabHandler.CompleteCourse)
		rehab.GET("/my-progress", rehabHandler.GetMyProgress)
	}

	adminHandler := handler.NewAdminHandler(db, cfg, rdb, ctr.AdminAuditService)
	adminHandler.SetNotificationService(notifSvc)
	adminFeedbackHandler := handler.NewAdminFeedbackHandler(db, ctr.FeedbackService, ctr.AdminAuditService)
	adminAuditHandler := handler.NewAdminAuditHandler(ctr.AdminAuditService)
	adminRAGHandler := handler.NewAdminRAGHandler(cfg, ctr.RAGProjection, ctr.AdminAuditService)
	adminArchiveScanHandler := handler.NewAdminArchiveScanHandler(db, ctr.ArchiveScanRepo, ctr.AdminAuditService, ctr.ArchiveObjectStore)
	adminArchiveScanHandler.SetArchiveScanCompletionNotifier(ctr.ReviewService)
	archiveScanAdminRateLimit := middleware.RedisFixedWindowLimit(rdb, "ratelimit:admin-archive-scan", cfg.RateLimit.NormalPerMinute, time.Minute, true)
	admin := v1.Group("/admin", authReq, middleware.AdminRequired())
	{
		admin.GET("/ips", adminHandler.ListPendingIPs)
		admin.POST("/ips/:id/approve", adminHandler.ApproveIP)
		admin.POST("/ips/:id/reject", adminHandler.RejectIP)
		admin.GET("/contents", adminHandler.ListUnderReviewContents)
		admin.GET("/contents/trash", adminHandler.ListTrashedContents)
		admin.POST("/contents/:id/ban", adminHandler.BanContent)
		admin.PATCH("/contents/:id/restore", adminHandler.RestoreContent)
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users/:id/ban", adminHandler.BanUser)
		admin.POST("/users/:id/unban", adminHandler.UnbanUser)
		admin.GET("/appeals", adminHandler.ListAppeals)
		admin.POST("/appeals/:id", adminHandler.ResolveAppeal)
		admin.GET("/reports", adminHandler.ListReports)
		admin.PATCH("/reports/:id", adminHandler.ResolveReport)
		admin.GET("/reports/stats", adminHandler.GetReportStats)
		admin.GET("/config", adminHandler.GetConfig)
		admin.PATCH("/config", adminHandler.PatchConfig)
		admin.POST("/judge/questions", judgeHandler.CreateQuestions)
		admin.POST("/categories", catHandler.AdminCreateCategory)
		admin.PATCH("/categories/:id", catHandler.AdminUpdateCategory)
		admin.DELETE("/categories/:id", catHandler.AdminDeleteCategory)
		admin.PUT("/categories/reorder", catHandler.AdminReorderCategories)
		admin.GET("/llm-configs", adminHandler.ListLLMConfigs)
		admin.POST("/llm-configs", adminHandler.CreateLLMConfig)
		admin.PATCH("/llm-configs/:id", adminHandler.UpdateLLMConfig)
		admin.DELETE("/llm-configs/:id", adminHandler.DeleteLLMConfig)
		admin.POST("/llm-configs/:id/activate", adminHandler.ActivateLLMConfig)
		admin.POST("/llm-configs/:id/test", adminHandler.TestLLMConfig)
		admin.GET("/queue/stats", adminHandler.GetQueueStats)
		admin.GET("/queue/dlq", adminHandler.GetDLQEntries)
		admin.POST("/queue/dlq/:id/replay", adminHandler.ReplayDLQEntry)
		admin.GET("/feedback", adminFeedbackHandler.ListFeedback)
		admin.GET("/feedback/:id", adminFeedbackHandler.GetFeedback)
		admin.PATCH("/feedback/:id", adminFeedbackHandler.PatchFeedback)
		admin.POST("/feedback/:id/replies", adminFeedbackHandler.ReplyFeedback)
		admin.POST("/notifications/broadcast", adminHandler.BroadcastNotification)
		admin.GET("/audit-logs", adminAuditHandler.ListAuditLogs)
		admin.POST("/rag/rebuild", adminRAGHandler.Rebuild)
		admin.GET("/archive-scan-jobs/:id", archiveScanAdminRateLimit, adminArchiveScanHandler.GetJob)
		admin.POST("/archive-scan-jobs/:id/manual-review", archiveScanAdminRateLimit, adminArchiveScanHandler.StartManualReview)
		admin.POST("/archive-scan-jobs/:id/resolve", archiveScanAdminRateLimit, adminArchiveScanHandler.ResolveManualReview)
		admin.POST("/archive-scan-jobs/:id/retry", archiveScanAdminRateLimit, adminArchiveScanHandler.Retry)
	}

	internalHandler := handler.NewInternalHandler(db, rdb, cfg)
	internalHandler.SetQueueProducer(ctr.QueueProducer)
	internal := v1.Group("/internal")
	{
		internal.POST("/ai-callback", internalHandler.AICallback)
	}

	v1.POST("/deploy-grants", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "desktop deploy is not enabled"})
	})

	v1.Any("/payments/*path", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "payment is not enabled"})
	})
}
