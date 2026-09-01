package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// FIX-02 (T02): the IP stats count must use the IP status vocabulary
// (pending/approved/rejected/banned) — "published" is a content status and
// made Active IPs permanently 0.
func TestStatsSummaryCountsApprovedIPsOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.IP{}, &model.ContentItem{}, &model.User{}))

	require.NoError(t, db.Create(&model.IP{Slug: "a1", Name: "approved-1", Status: "approved"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "a2", Name: "approved-2", Status: "approved"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "p1", Name: "pending", Status: "pending"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "r1", Name: "rejected", Status: "rejected"}).Error)
	require.NoError(t, db.Create(&model.IP{Slug: "b1", Name: "banned", Status: "banned"}).Error)
	// contents keep the published vocabulary — must not affect the IP count
	require.NoError(t, db.Create(&model.ContentItem{Title: "c1", Status: "published"}).Error)

	svc := NewStatsService(db, nil)
	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(2), summary.IPs, "only approved IPs count toward Active IPs")
	require.Equal(t, int64(1), summary.Contents, "content stats keep the published status word")
}
