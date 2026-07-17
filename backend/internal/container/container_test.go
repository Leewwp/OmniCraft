package container

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/config"
)

func TestNewContainerOwnsRouteLevelDomainServices(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	ctr := NewContainer(db, nil, &config.Config{})
	if ctr.StatsService == nil {
		t.Fatal("NewContainer must construct StatsService for the HTTP composition root")
	}
	if ctr.IPStatsService == nil {
		t.Fatal("NewContainer must retain IPStatsService ownership")
	}
	if ctr.SearchService == nil {
		t.Fatal("NewContainer must retain SearchService ownership")
	}
}
