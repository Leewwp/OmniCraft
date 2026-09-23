package container

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
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

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctr, err := NewContainer(db, rdb, &config.Config{})
	if err != nil {
		t.Fatalf("NewContainer: %v", err)
	}
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
