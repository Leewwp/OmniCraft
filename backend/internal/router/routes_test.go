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
