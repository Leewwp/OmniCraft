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
		`contents.POST("", authReq, publishGuard, middleware.UploadRateLimit(rdb, &cfg.RateLimit), contentHandler.CreateContent)`,
		`admin := v1.Group("/admin", authReq, middleware.AdminRequired())`,
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
	contract := `rehabHandler := handler.NewRehabHandler(db, rdb, cfg)`
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

func TestSeriesMutationRoutesUseAuthAndStandardInteractionGuard(t *testing.T) {
	source := readRoutesSource(t)
	contracts := []string{
		`seriesGuard := middleware.InteractionRequired(cfg, db, rdb, standardVerifiedInteractionPolicy())`,
		`v1.POST("/series", authReq, seriesGuard, seriesHandler.CreateSeries)`,
		`v1.PUT("/series/:id", authReq, seriesGuard, seriesHandler.UpdateSeries)`,
		`v1.DELETE("/series/:id", authReq, seriesGuard, seriesHandler.DeleteSeries)`,
		`v1.POST("/series/:id/items", authReq, seriesGuard, seriesHandler.AddItem)`,
		`v1.DELETE("/series/:id/items/:itemId", authReq, seriesGuard, seriesHandler.RemoveItem)`,
		`v1.PUT("/series/:id/items/reorder", authReq, seriesGuard, seriesHandler.ReorderItems)`,
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
