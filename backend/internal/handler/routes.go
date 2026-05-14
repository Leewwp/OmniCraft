package handler

import (
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
	authHandler := NewAuthHandler(authService, userRepo, rdb)

	notifSvc := ctr.NotificationService

	auth := v1.Group("/auth")
	{
		auth.POST("/register", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), authHandler.Register)
		auth.POST("/login", middleware.CredentialRateLimit(rdb, &cfg.RateLimit), authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/refresh", authHandler.Refresh)
		auth.GET("/me", middleware.AuthRequired(cfg, rdb), authHandler.Me)
	}

	optAuth := middleware.OptionalAuth(cfg, rdb)

	userHandler := NewUserHandler(db, authService, rdb, cfg)
	users := v1.Group("/users")
	{
		users.GET("/:id", optAuth, userHandler.GetUser)
		users.PATCH("/:id", middleware.AuthRequired(cfg, rdb), userHandler.UpdateUser)
		users.GET("/:id/reputation", optAuth, userHandler.GetReputation)
		users.GET("/:id/contents", optAuth, userHandler.GetUserContents)
		users.DELETE("/me", middleware.AuthRequired(cfg, rdb), userHandler.DeleteAccount)
		users.PATCH("/me/password", middleware.AuthRequired(cfg, rdb), userHandler.ChangePassword)
		users.PATCH("/me/support-info", middleware.AuthRequired(cfg, rdb), userHandler.UpdateSupportInfo)
	}

	ipHandler := NewIPHandlerWithCache(db, rdb, cfg)
	ips := v1.Group("/ips")
	{
		ips.GET("", optAuth, ipHandler.ListIPs)
		ips.POST("", middleware.AuthRequired(cfg, rdb), ipHandler.CreateIP)
		ips.GET("/:id", optAuth, ipHandler.GetIP)
		ips.GET("/:id/contents", optAuth, ipHandler.GetIPContents)
	}

	contentHandler := NewContentHandler(db, cfg, rdb)
	contents := v1.Group("/contents")
	{
		contents.GET("", optAuth, contentHandler.ListContents)
		contents.POST("", middleware.AuthRequired(cfg, rdb), middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)
		contents.POST("/oss-token", middleware.AuthRequired(cfg, rdb), middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.GenerateOSSToken)
		contents.GET("/:id/related-fanworks", optAuth, contentHandler.ListRelatedFanworks)
		contents.GET("/:id", optAuth, contentHandler.GetContent)
		contents.PATCH("/:id", middleware.AuthRequired(cfg, rdb), contentHandler.UpdateContent)
		contents.DELETE("/:id", middleware.AuthRequired(cfg, rdb), contentHandler.DeleteContent)
		contents.GET("/:id/versions", optAuth, NewVersionHandler(db).ListVersions)
		contents.GET("/:id/prs", optAuth, NewPRHandler(db).ListPRs)
			contents.GET("/:id/download", middleware.AuthRequired(cfg, rdb), contentHandler.DownloadContent)
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
		pr.POST("", middleware.AuthRequired(cfg, rdb), prHandler.SubmitPR)
		pr.GET("/:id", optAuth, prHandler.GetPR)
		pr.POST("/:id/accept", middleware.AuthRequired(cfg, rdb), prHandler.AcceptPR)
		pr.POST("/:id/reject", middleware.AuthRequired(cfg, rdb), prHandler.RejectPR)
		pr.POST("/:id/merge", middleware.AuthRequired(cfg, rdb), prHandler.ManualMerge)
	}

	dashboard := v1.Group("/dashboard", middleware.AuthRequired(cfg, rdb))
	{
		dashboard.POST("/contributors/:userId/block", prHandler.BlockContributor)
		dashboard.DELETE("/contributors/:userId/block", prHandler.UnblockContributor)
	}

	socialSvc := ctr.SocialService
	socialHandler := &SocialHandler{socialSvc: socialSvc}
	social := v1.Group("/social")
	{
		social.GET("/comments", optAuth, socialHandler.ListComments)
		social.POST("/comments", middleware.AuthRequired(cfg, rdb), socialHandler.PostComment)
		social.DELETE("/comments/:id", middleware.AuthRequired(cfg, rdb), socialHandler.DeleteComment)
		social.GET("/discussions", optAuth, socialHandler.ListDiscussions)
		social.POST("/discussions", middleware.AuthRequired(cfg, rdb), socialHandler.PostDiscussion)
		social.GET("/discussions/:id", optAuth, socialHandler.GetDiscussion)
		social.POST("/reactions", middleware.AuthRequired(cfg, rdb), socialHandler.React)
		social.POST("/comments/:id/report", middleware.AuthRequired(cfg, rdb), socialHandler.ReportComment)
	}
	contents.POST("/:id/report", middleware.AuthRequired(cfg, rdb), socialHandler.ReportContent)

	favHandler := NewFavoriteHandler(db, cfg)
	favorites := v1.Group("/favorites", middleware.AuthRequired(cfg, rdb))
	{
		favorites.POST("", favHandler.AddFavorite)
		favorites.DELETE("/:contentId", favHandler.RemoveFavorite)
	}
	users.GET("/:id/favorites", optAuth, favHandler.ListUserFavorites)

	judgeHandler := NewJudgeHandler(db, cfg)
	judge := v1.Group("/judge")
	{
		judge.GET("/exam/:category", optAuth, judgeHandler.GetExam)
		judge.POST("/exam/submit", middleware.AuthRequired(cfg, rdb), judgeHandler.SubmitExam)
		judge.GET("/queue", middleware.AuthRequired(cfg, rdb), judgeHandler.GetQueue)
		judge.POST("/vote", middleware.AuthRequired(cfg, rdb), judgeHandler.SubmitVote)
		judge.GET("/cases/:id/verdict", optAuth, judgeHandler.GetVerdictDetail)
		judge.POST("/reasons/:id/vote", middleware.AuthRequired(cfg, rdb), judgeHandler.VoteReason)
	}

	ipStatsSvc := service.NewIPStatsService(db, rdb)
	ipStatsHandler := NewIPStatsHandler(ipStatsSvc)
	v1.GET("/ips/stats/category_counts", ipStatsHandler.GetCategoryCounts)

	catHandler := NewCategoryHandler(db)
	v1.GET("/categories", optAuth, catHandler.ListCategories)

	tagHandler := NewTagHandler(db, rdb)
	v1.GET("/tags/faceted", optAuth, tagHandler.GetFacetedTags)
	v1.GET("/tags/search", optAuth, tagHandler.SearchTags)
	contents.POST("/:id/tags/suggest", middleware.AuthRequired(cfg, rdb), tagHandler.SuggestTag)
	dashboard.GET("/tag-suggestions", tagHandler.ListTagSuggestions)
	dashboard.PATCH("/tag-suggestions/:id", tagHandler.UpdateTagSuggestion)

	me := v1.Group("/users/me", middleware.AuthRequired(cfg, rdb))
	{
		me.GET("/tag-groups", tagHandler.ListTagGroups)
		me.POST("/tag-groups", tagHandler.CreateTagGroup)
		me.PATCH("/tag-groups/:id", tagHandler.UpdateTagGroup)
		me.DELETE("/tag-groups/:id", tagHandler.DeleteTagGroup)
		me.GET("/saved-searches", tagHandler.ListSavedSearches)
		me.POST("/saved-searches", tagHandler.CreateSavedSearch)
		me.DELETE("/saved-searches/:id", tagHandler.DeleteSavedSearch)
	}

	searchHandler := NewSearchHandler(service.NewSearchService(repository.NewSearchRepository(db), rdb))
	v1.GET("/search/suggestions", searchHandler.Suggestions)
	v1.GET("/search/trending", searchHandler.Trending)

	followHandler := NewFollowHandler(db)
	followHandler.SetNotificationService(notifSvc)
	users.POST("/:id/follow", middleware.AuthRequired(cfg, rdb), followHandler.FollowUser)
	users.DELETE("/:id/follow", middleware.AuthRequired(cfg, rdb), followHandler.UnfollowUser)
	users.GET("/:id/followers", optAuth, followHandler.GetFollowers)
	users.GET("/:id/following", optAuth, followHandler.GetFollowing)
	ips.POST("/:id/follow", middleware.AuthRequired(cfg, rdb), followHandler.FollowIP)
	ips.DELETE("/:id/follow", middleware.AuthRequired(cfg, rdb), followHandler.UnfollowIP)

	appealHandler := NewAppealHandler(db)
	v1.POST("/appeals", middleware.AuthRequired(cfg, rdb), appealHandler.SubmitAppeal)
	v1.GET("/appeals/me", middleware.AuthRequired(cfg, rdb), appealHandler.GetMyAppeals)

	notifHandler := NewNotificationHandler(db)
	notif := v1.Group("/notifications", middleware.AuthRequired(cfg, rdb))
	{
		notif.GET("", notifHandler.ListNotifications)
		notif.PATCH("/:id/read", notifHandler.MarkRead)
		notif.POST("/read-all", notifHandler.MarkAllRead)
		notif.GET("/unread-count", notifHandler.UnreadCount)
	}

	msgHandler := NewMessageHandler(db)
	msgHandler.SetNotificationService(notifSvc)
	messages := v1.Group("/messages", middleware.AuthRequired(cfg, rdb))
	{
		messages.GET("", msgHandler.ListConversations)
		messages.POST("", msgHandler.SendMessage)
		messages.GET("/:id", msgHandler.ListMessages)
	}

	histHandler := NewBrowseHistoryHandler(db)
	me.POST("/history", histHandler.RecordView)
	me.GET("/history", histHandler.GetHistory)
	me.DELETE("/history", histHandler.ClearHistory)

	discHandler := NewDiscussionHandler(db)
	ips.GET("/:id/discussions", optAuth, discHandler.ListDiscussions)
	ips.POST("/:id/discussions", middleware.AuthRequired(cfg, rdb), discHandler.CreateDiscussion)
	ips.GET("/:id/discussions/search", optAuth, discHandler.SearchDiscussions)
	users.GET("/:id/discussions", optAuth, discHandler.ListByUser)
	discussions := v1.Group("/discussions")
	{
		discussions.GET("/:id", optAuth, discHandler.GetDiscussion)
		discussions.POST("/:id/comments", middleware.AuthRequired(cfg, rdb), discHandler.ReplyToDiscussion)
		discussions.PATCH("/:id/pin", middleware.AuthRequired(cfg, rdb), discHandler.PinDiscussion)
	}

	repHandler := NewReputationHandler(db)
	v1.GET("/reputation-logs/me", middleware.AuthRequired(cfg, rdb), repHandler.GetMyReputationLogs)

	agentHandler := NewAgentHandler(db, cfg)
	agent := v1.Group("/agent", middleware.AuthRequired(cfg, rdb), middleware.AgentRateLimit(rdb, cfg))
	{
		agent.POST("/upload-assist", agentHandler.UploadAssist)
		agent.POST("/compliance-check", agentHandler.ComplianceCheck)
		agent.POST("/search", agentHandler.NLSearch)
		agent.POST("/usage-guide/:id", agentHandler.UsageGuide)
		agent.POST("/moderate/:id", agentHandler.Moderate)
		agent.POST("/chat/stream", agentHandler.ChatStream)
		agent.GET("/script/:id", agentHandler.GenerateDeployScript)
	}

	rehabHandler := NewRehabHandler(db)
	rehab := v1.Group("/rehab", middleware.AuthRequired(cfg, rdb))
	{
		rehab.GET("/courses", rehabHandler.ListCourses)
		rehab.GET("/courses/:id", rehabHandler.GetCourse)
		rehab.POST("/courses/:id/start", rehabHandler.StartCourse)
		rehab.POST("/courses/:id/complete", rehabHandler.CompleteCourse)
		rehab.GET("/my-progress", rehabHandler.GetMyProgress)
	}

	adminHandler := NewAdminHandler(db, cfg, rdb)
	adminHandler.SetNotificationService(notifSvc)
	admin := v1.Group("/admin", middleware.AuthRequired(cfg, rdb), middleware.AdminRequired())
	{
		admin.GET("/ips", adminHandler.ListPendingIPs)
		admin.POST("/ips/:id/approve", adminHandler.ApproveIP)
		admin.POST("/ips/:id/reject", adminHandler.RejectIP)
		admin.GET("/contents", adminHandler.ListUnderReviewContents)
		admin.POST("/contents/:id/ban", adminHandler.BanContent)
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users/:id/ban", adminHandler.BanUser)
		admin.POST("/users/:id/unban", adminHandler.UnbanUser)
		admin.GET("/appeals", adminHandler.ListAppeals)
		admin.POST("/appeals/:id", adminHandler.ResolveAppeal)
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
	}

	internalHandler := NewInternalHandler(db, rdb, cfg)
	internal := v1.Group("/internal")
	{
		internal.POST("/ai-callback", internalHandler.AICallback)
	}
}
