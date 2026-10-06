package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterIsSoleRouteOwner(t *testing.T) {
	source := readRoutesSource(t)
	if !strings.Contains(source, "func RegisterRoutes(") {
		t.Fatal("internal/router must own RegisterRoutes")
	}

	handlerSource, err := os.ReadFile(filepath.Join("..", "handler", "routes.go"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read legacy handler route owner: %v", err)
	}
	if strings.Contains(string(handlerSource), "func RegisterRoutes(") {
		t.Fatal("internal/handler must not retain a second RegisterRoutes owner")
	}
}

func TestRouterUsesOnlyContainerOwnedDomainDependencies(t *testing.T) {
	source := readRoutesSource(t)
	for _, forbidden := range []string{"repository.New", "service.New"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("router composition must not construct repository/domain services with %q", forbidden)
		}
	}
}

func TestRouterSourcePreservesRepresentativeRouteContracts(t *testing.T) {
	source := readRoutesSource(t)
	contracts := []string{
		`v1.GET("/config/public", publicConfigHandler.GetPublicConfig)`,
		`contents.POST("", authReq, middleware.RequireScopeForPAT("upload"), publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)`,
		// run-1 审计 #12：admin 面（组 + 组外 pin）仅限 web JWT 通道——泄露的
		// admin PAT 不得成为全权机器通道。
		`admin := v1.Group("/admin", authReq, middleware.RequireJWTChannel(), middleware.AdminRequired())`,
		`discussions.PATCH("/:id/pin", authReq, middleware.RequireJWTChannel(), middleware.AdminRequired(), discHandler.PinDiscussion)`,
		`v1.POST("/deploy-grants", func(c *gin.Context)`,
		`c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "desktop deploy is not enabled"})`,
		`v1.Any("/payments/*path", func(c *gin.Context)`,
		`c.JSON(http.StatusServiceUnavailable, gin.H{"code": "FEATURE_DISABLED", "message": "payment is not enabled"})`,
	}
	for _, contract := range contracts {
		if !strings.Contains(source, contract) {
			t.Errorf("router source missing route contract %q", contract)
		}
	}
}

func TestRehabHandlerReceivesRuntimeStatusDependencies(t *testing.T) {
	source := readRoutesSource(t)
	contract := `rehabHandler := handler.NewRehabHandler(ctr.RehabService)`
	if !strings.Contains(source, contract) {
		t.Fatalf("router source missing rehab cache invalidation wiring %q", contract)
	}
}

func TestIPVisitHistoryRoutesRequireAuthUnderUsersMe(t *testing.T) {
	source := readRoutesSource(t)
	contracts := []string{
		`me.GET("/ip-visits", ipVisitHistoryHandler.ListRecent)`,
		`me.PUT("/ip-visits/:ipId", ipVisitHistoryHandler.RecordVisit)`,
		`me.POST("/ip-visits/merge", ipVisitHistoryHandler.MergeVisits)`,
		`me := v1.Group("/users/me", authReq)`,
	}
	for _, contract := range contracts {
		if !strings.Contains(source, contract) {
			t.Errorf("router source missing IP visit history route contract %q", contract)
		}
	}
	if strings.Count(source, `me := v1.Group("/users/me", authReq)`) != 1 {
		t.Error("IP visit history routes must be registered on the single auth-required users/me group")
	}
}

func TestLegacyFavoritesRoutesAreNotRegistered(t *testing.T) {
	source := readRoutesSource(t)
	for _, legacy := range []string{
		`favorites := v1.Group("/favorites", authReq)`,
		`favorites.POST("", favoritesGuard, favHandler.AddFavorite)`,
		`favorites.DELETE("/:contentId", favoritesGuard, favHandler.RemoveFavorite)`,
		`users.GET("/:id/favorites", optAuth, favHandler.ListUserFavorites)`,
		`favHandler := handler.NewFavoriteHandler(db, cfg)`,
		`favoritesGuard := middleware.InteractionRequired`,
	} {
		if strings.Contains(source, legacy) {
			t.Errorf("router source still registers legacy favorites dependency %q", legacy)
		}
	}

	collectionContracts := []string{
		`collectionGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())`,
		// run-1 审计 #11：collections 变更族挂 PAT upload scope 闸（JWT 不受影响）。
		`v1.POST("/collections", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.CreateCollection)`,
		`v1.PUT("/collections/:id", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.UpdateCollection)`,
		`v1.DELETE("/collections/:id", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.DeleteCollection)`,
		`v1.POST("/collections/:id/items", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.AddItem)`,
		`v1.DELETE("/collections/:id/items/:itemId", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.RemoveItem)`,
		`v1.PUT("/collections/:id/items/:itemId", authReq, middleware.RequireScopeForPAT("upload"), collectionGuard, collectionHandler.UpdateItem)`,
	}
	for _, contract := range collectionContracts {
		if !strings.Contains(source, contract) {
			t.Errorf("router source missing collection route interaction-guard contract %q", contract)
		}
	}
}

func TestSeriesMutationRoutesUseAuthAndStandardInteractionGuard(t *testing.T) {
	source := readRoutesSource(t)
	contracts := []string{
		`seriesGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())`,
		// run-1 审计 #11：六条 series 变更路由挂 PAT upload scope 闸
		//（series 变更=内容组织写；download-only PAT 此前可硬删系列）。
		`v1.POST("/series", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.CreateSeries)`,
		`v1.PUT("/series/:id", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.UpdateSeries)`,
		`v1.DELETE("/series/:id", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.DeleteSeries)`,
		`v1.POST("/series/:id/items", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.AddItem)`,
		`v1.DELETE("/series/:id/items/:itemId", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.RemoveItem)`,
		`v1.PUT("/series/:id/items/reorder", authReq, middleware.RequireScopeForPAT("upload"), seriesGuard, seriesHandler.ReorderItems)`,
		`v1.GET("/series", authReq, seriesHandler.ListSeries)`,
		`v1.GET("/series/candidates", authReq, seriesHandler.ListCandidates)`,
		`v1.GET("/series/:id", optAuth, seriesHandler.GetSeries)`,
	}
	for _, contract := range contracts {
		if !strings.Contains(source, contract) {
			t.Errorf("router source missing series route contract %q", contract)
		}
	}
}
