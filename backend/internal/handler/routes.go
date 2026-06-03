package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/container"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func RegisterRoutes(v1 *gin.RouterGroup, cfg *config.Config, ctr *container.ServiceContainer) {
	rdb := ctr.RDB
	db := ctr.DB
	userRepo := ctr.UserRepo
	authService := ctr.AuthService
	authHandler := NewAuthHandler(authService, ctr.VerificationService, userRepo, ctr.CaptchaVerifier, rdb, cfg)

	notifSvc := ctr.NotificationService

	optAuth := middleware.OptionalAuth(cfg, rdb, db)
	authReq := middleware.AuthRequired(cfg, rdb, db)

	publishGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail:   true,
		RequireReputation:      true,
		RequireNoPublishFreeze: true,
	})
	editDeleteGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	commentsGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	reactionsGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	favoritesGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	reportsGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	prGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	judgeGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	followsGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	messagesGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	downloadsGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})
	agentGuard := middleware.InteractionRequired(cfg, db, rdb, middleware.InteractionPolicy{
		RequireVerifiedEmail: true,
		RequireReputation:    true,
	})

	publicConfigHandler := NewPublicConfigHandler(cfg)
	v1.GET("/config/public", publicConfigHandler.GetPublicConfig)

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

	userHandler := NewUserHandler(db, authService, rdb, cfg)
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

	ipHandler := NewIPHandlerWithCache(db, rdb, cfg)
	ips := v1.Group("/ips")
	{
		ips.GET("", optAuth, ipHandler.ListIPs)
		ips.POST("", authReq, ipHandler.CreateIP)
		ips.GET("/:id", optAuth, ipHandler.GetIP)
		ips.GET("/:id/contents", optAuth, ipHandler.GetIPContents)
	}

	contentHandler := NewContentHandler(db, cfg, rdb)
	contentHandler.SetQueueProducer(ctr.QueueProducer)
	contents := v1.Group("/contents")
	{
		contents.GET("", optAuth, contentHandler.ListContents)
		contents.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)
		contents.POST("/oss-token", authReq, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.GenerateOSSToken)
		contents.GET("/:id/related-fanworks", optAuth, contentHandler.ListRelatedFanworks)
		contents.GET("/:id", optAuth, contentHandler.GetContent)
		contents.PATCH("/:id", authReq, editDeleteGuard, contentHandler.UpdateContent)
		contents.DELETE("/:id", authReq, editDeleteGuard, contentHandler.DeleteContent)
		contents.GET("/:id/versions", optAuth, NewVersionHandler(db).ListVersions)
		contents.GET("/:id/prs", optAuth, NewPRHandler(db).ListPRs)
		contents.GET("/:id/download", authReq, downloadsGuard, contentHandler.DownloadContent)
	}

	versionHandler := NewVersionHandler(db)
	versions := v1.Group("/versions")
	{
		versions.GET("/:id", optAuth, versionHandler.GetVersion)
	}

	prSvc := ctr.PRService
	prHandler := &PRHandler{prSvc: prSvc}
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
	socialHandler := &SocialHandler{socialSvc: socialSvc}
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
	}
	contents.POST("/:id/report", authReq, reportsGuard, socialHandler.ReportContent)

	favHandler := NewFavoriteHandler(db, cfg)
	favorites := v1.Group("/favorites", authReq)
	{
		favorites.POST("", favoritesGuard, favHandler.AddFavorite)
		favorites.DELETE("/:contentId", favoritesGuard, favHandler.RemoveFavorite)
	}
	users.GET("/:id/favorites", optAuth, favHandler.ListUserFavorites)

	judgeHandler := NewJudgeHandler(db, cfg, ctr.AdminAuditService)
	judge := v1.Group("/judge")
	{
		judge.GET("/exam/:category", optAuth, judgeHandler.GetExam)
		judge.POST("/exam/submit", authReq, judgeGuard, judgeHandler.SubmitExam)
		judge.GET("/queue", authReq, judgeHandler.GetQueue)
		judge.POST("/vote", authReq, judgeGuard, judgeHandler.SubmitVote)
		judge.GET("/cases/:id/verdict", optAuth, judgeHandler.GetVerdictDetail)
		judge.POST("/reasons/:id/vote", authReq, judgeGuard, judgeHandler.VoteReason)
	}

	statsSvc := service.NewStatsService(db, rdb)
	statsHandler := NewStatsHandler(statsSvc)
	v1.GET("/stats/summary", optAuth, statsHandler.GetSummary)

	ipStatsSvc := service.NewIPStatsService(db, rdb)
	ipStatsHandler := NewIPStatsHandler(ipStatsSvc)
	v1.GET("/ips/stats/category_counts", optAuth, ipStatsHandler.GetCategoryCounts)

	catHandler := NewCategoryHandler(db, ctr.AdminAuditService)
	v1.GET("/categories", optAuth, catHandler.ListCategories)

	tagHandler := NewTagHandler(db, rdb, &cfg.Cache)
	v1.GET("/tags/faceted", optAuth, tagHandler.GetFacetedTags)
	v1.GET("/tags/search", optAuth, tagHandler.SearchTags)
	contents.POST("/:id/tags/suggest", authReq, tagHandler.SuggestTag)
	dashboard.GET("/tag-suggestions", tagHandler.ListTagSuggestions)
	dashboard.PATCH("/tag-suggestions/:id", tagHandler.UpdateTagSuggestion)

	followHandler := NewFollowHandler(db)
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
	}

	searchHandler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb))
	v1.GET("/search/suggestions", optAuth, searchHandler.Suggestions)
	v1.GET("/search/trending", optAuth, searchHandler.Trending)
	v1.GET("/contents/search", optAuth, searchHandler.SearchContents)

	users.POST("/:id/follow", authReq, followsGuard, followHandler.FollowUser)
	users.DELETE("/:id/follow", authReq, followsGuard, followHandler.UnfollowUser)
	users.GET("/:id/followers", optAuth, followHandler.GetFollowers)
	users.GET("/:id/following", optAuth, followHandler.GetFollowing)
	users.GET("/search", optAuth, searchHandler.SearchUsers)
	ips.POST("/:id/follow", authReq, followsGuard, followHandler.FollowIP)
	ips.DELETE("/:id/follow", authReq, followsGuard, followHandler.UnfollowIP)

	feedbackHandler := NewFeedbackHandler(ctr.FeedbackService)
	feedback := v1.Group("/feedback")
	{
		feedback.POST("", optAuth, feedbackHandler.SubmitTicket)
		feedback.POST("/attachments/presign", optAuth, feedbackHandler.PresignUpload)
		feedback.PUT("/attachments/staging/:grant_id", optAuth, feedbackHandler.StageUpload)
		feedback.GET("/me", authReq, feedbackHandler.ListMyTickets)
		feedback.GET("/:id", authReq, feedbackHandler.GetTicket)
	}

	appealHandler := NewAppealHandler(db)
	v1.POST("/appeals", authReq, appealHandler.SubmitAppeal)
	v1.GET("/appeals/me", authReq, appealHandler.GetMyAppeals)

	notifHandler := NewNotificationHandler(db)
	notif := v1.Group("/notifications", authReq)
	{
		notif.GET("", notifHandler.ListNotifications)
		notif.PATCH("/:id/read", notifHandler.MarkRead)
		notif.POST("/read-all", notifHandler.MarkAllRead)
		notif.GET("/unread-count", notifHandler.UnreadCount)
	}

	msgHandler := NewMessageHandler(db)
	msgHandler.SetNotificationService(notifSvc)
	messages := v1.Group("/messages", authReq)
	{
		messages.GET("", messagesGuard, msgHandler.ListConversations)
		messages.POST("", messagesGuard, msgHandler.SendMessage)
		messages.GET("/:id", msgHandler.ListMessages)
		messages.DELETE("/:id", msgHandler.DeleteMessage)
		messages.DELETE("/conversations/:id", msgHandler.LeaveConversation)
	}

	histHandler := NewBrowseHistoryHandler(db)
	me.POST("/history", histHandler.RecordView)
	me.GET("/history", histHandler.GetHistory)
	me.DELETE("/history", histHandler.ClearHistory)

	discHandler := NewDiscussionHandler(db)
	ips.GET("/:id/discussions", optAuth, discHandler.ListDiscussions)
	ips.POST("/:id/discussions", authReq, discHandler.CreateDiscussion)
	ips.GET("/:id/discussions/search", optAuth, discHandler.SearchDiscussions)
	users.GET("/:id/discussions", optAuth, discHandler.ListByUser)
	discussions := v1.Group("/discussions")
	{
		discussions.GET("/:id", optAuth, discHandler.GetDiscussion)
		discussions.POST("/:id/comments", authReq, commentsGuard, discHandler.ReplyToDiscussion)
		discussions.PATCH("/:id/pin", authReq, discHandler.PinDiscussion)
	}

	repHandler := NewReputationHandler(db)
	v1.GET("/reputation-logs/me", authReq, repHandler.GetMyReputationLogs)

	agentHandler := NewAgentHandler(db, cfg)
	agentHandler.SetQueueProducer(ctr.QueueProducer)
	agent := v1.Group("/agent", authReq, agentGuard, middleware.AgentRateLimit(rdb, cfg))
	{
		agent.POST("/upload-assist", agentHandler.UploadAssist)
		agent.POST("/compliance-check", agentHandler.ComplianceCheck)
		agent.POST("/search", agentHandler.NLSearch)
		agent.GET("/usage-guide/:id", agentHandler.UsageGuide)
		agent.POST("/moderate/:id", agentHandler.Moderate)
		agent.POST("/chat/stream", agentHandler.ChatStream)
		agent.GET("/conversations", agentHandler.ListConversations)
		agent.GET("/conversations/:id", agentHandler.GetConversationMessages)
	}

	rehabHandler := NewRehabHandler(db)
	rehab := v1.Group("/rehab", authReq)
	{
		rehab.GET("/courses", rehabHandler.ListCourses)
		rehab.GET("/courses/:id", rehabHandler.GetCourse)
		rehab.POST("/courses/:id/start", rehabHandler.StartCourse)
		rehab.POST("/courses/:id/complete", rehabHandler.CompleteCourse)
		rehab.GET("/my-progress", rehabHandler.GetMyProgress)
	}

	adminHandler := NewAdminHandler(db, cfg, rdb, ctr.AdminAuditService)
	adminHandler.SetNotificationService(notifSvc)
	adminFeedbackHandler := NewAdminFeedbackHandler(ctr.FeedbackService, ctr.AdminAuditService)
	adminAuditHandler := NewAdminAuditHandler(ctr.AdminAuditService)
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
		admin.GET("/feedback", adminFeedbackHandler.ListFeedback)
		admin.GET("/feedback/:id", adminFeedbackHandler.GetFeedback)
		admin.PATCH("/feedback/:id", adminFeedbackHandler.PatchFeedback)
		admin.POST("/feedback/:id/replies", adminFeedbackHandler.ReplyFeedback)
		admin.GET("/audit-logs", adminAuditHandler.ListAuditLogs)
	}

	internalHandler := NewInternalHandler(db, rdb, cfg)
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
