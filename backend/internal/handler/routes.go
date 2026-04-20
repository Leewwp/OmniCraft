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
}
