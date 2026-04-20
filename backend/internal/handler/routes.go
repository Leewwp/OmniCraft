package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
)

func RegisterRoutes(v1 *gin.RouterGroup, cfg *config.Config, db *gorm.DB, rdb *redis.Client) {
	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, rdb, cfg)
	authHandler := NewAuthHandler(authService, userRepo)

	auth := v1.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/logout", authHandler.Logout)
		auth.POST("/refresh", authHandler.Refresh)
		auth.GET("/me", middleware.AuthRequired(cfg), authHandler.Me)
	}

	userHandler := NewUserHandler(db)
	users := v1.Group("/users")
	{
		users.GET("/:id", userHandler.GetUser)
		users.PATCH("/:id", middleware.AuthRequired(cfg), userHandler.UpdateUser)
		users.GET("/:id/reputation", userHandler.GetReputation)
	}

	ipHandler := NewIPHandler(db)
	ips := v1.Group("/ips")
	{
		ips.GET("", ipHandler.ListIPs)
		ips.POST("", middleware.AuthRequired(cfg), ipHandler.CreateIP)
		ips.GET("/:id", ipHandler.GetIP)
		ips.GET("/:id/contents", ipHandler.GetIPContents)
		ips.GET("/:id/discussions", ipHandler.GetIPDiscussions)
	}

	contentHandler := NewContentHandler(db)
	contents := v1.Group("/contents")
	{
		contents.GET("", contentHandler.ListContents)
		contents.POST("", middleware.AuthRequired(cfg), contentHandler.CreateContent)
		contents.GET("/:id", contentHandler.GetContent)
		contents.PATCH("/:id", middleware.AuthRequired(cfg), contentHandler.UpdateContent)
		contents.DELETE("/:id", middleware.AuthRequired(cfg), contentHandler.DeleteContent)
		contents.GET("/:id/versions", NewVersionHandler(db).ListVersions)
		contents.GET("/:id/prs", NewPRHandler(db).ListPRs)
	}

	versionHandler := NewVersionHandler(db)
	versions := v1.Group("/versions")
	{
		versions.GET("/:id", versionHandler.GetVersion)
	}

	prHandler := NewPRHandler(db)
	pr := v1.Group("/pr")
	{
		pr.POST("", middleware.AuthRequired(cfg), prHandler.SubmitPR)
		pr.GET("/:id", prHandler.GetPR)
		pr.POST("/:id/accept", middleware.AuthRequired(cfg), prHandler.AcceptPR)
		pr.POST("/:id/reject", middleware.AuthRequired(cfg), prHandler.RejectPR)
	}
}
