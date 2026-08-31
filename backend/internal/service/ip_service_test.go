package service

import (
	"context"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// B3: the IP review input carries the IP cover URL, keeps the pure text
// synchronous path (no attachments, no async callback for IP).
func TestIPReviewInputCarriesCoverURL(t *testing.T) {
	ip := &model.IP{
		ID:          7,
		Name:        "示例 IP",
		Slug:        "example-ip",
		Description: "desc",
		CoverURL:    "https://cdn.example.test/uploads/42/ip/cover.png",
	}
	in := ipReviewInput(ip, 42)

	require.Equal(t, "ip", in.TargetType)
	require.Equal(t, int64(7), in.TargetID)
	require.Equal(t, "示例 IP", in.Title)
	require.Equal(t, "desc", in.Description)
	require.Equal(t, int64(42), in.AuthorID)
	require.Equal(t, ip.CoverURL, in.CoverImageURL, "IP cover must enter the image review input")
	require.Empty(t, in.Attachments, "IP has no attachments; the fact is preserved")
}

func TestIPReviewInputEmptyCoverStaysEmpty(t *testing.T) {
	ip := &model.IP{ID: 9, Name: "no cover", Slug: "no-cover"}
	in := ipReviewInput(ip, 1)
	require.Empty(t, in.CoverImageURL)
}

// B-003: CreateIP must persist tags into ip_tags and GetIP must join them
// back onto the response.
func TestCreateIPPersistsTagsAndGetIPReturnsThem(t *testing.T) {
	db := setupIPServiceDB(t)
	creator := seedIPServiceUser(t, db, 1, "ip-tag-creator")
	svc := NewIPService(repository.NewIPRepository(db))

	ip, err := svc.CreateIP(context.Background(), CreateIPInput{
		Name:        "Tagged IP",
		Description: "desc",
		Category:    "game",
		Tags:        []string{"奇幻", "冒险", "奇幻", "  ", ""},
	}, creator.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"奇幻", "冒险"}, ip.Tags, "create response should carry normalized tags")

	var rows []model.IPTag
	require.NoError(t, db.Where("ip_id = ?", ip.ID).Order("tag ASC").Find(&rows).Error)
	require.Len(t, rows, 2, "ip_tags must hold one row per deduped tag")

	fetched, err := svc.GetIP(ip.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"冒险", "奇幻"}, fetched.Tags, "GetIP returns tags ordered by ip_tags.tag ASC")
}

func TestCreateIPWithoutTagsWritesNoTagRows(t *testing.T) {
	db := setupIPServiceDB(t)
	creator := seedIPServiceUser(t, db, 2, "ip-no-tag-creator")
	svc := NewIPService(repository.NewIPRepository(db))

	ip, err := svc.CreateIP(context.Background(), CreateIPInput{Name: "Plain IP"}, creator.ID)
	require.NoError(t, err)
	require.Empty(t, ip.Tags)

	var count int64
	require.NoError(t, db.Model(&model.IPTag{}).Where("ip_id = ?", ip.ID).Count(&count).Error)
	require.Zero(t, count)

	fetched, err := svc.GetIP(ip.ID)
	require.NoError(t, err)
	require.Empty(t, fetched.Tags)
}

func TestNormalizeIPTagsTrimsDedupesAndTruncates(t *testing.T) {
	require.Nil(t, normalizeIPTags(nil))
	require.Nil(t, normalizeIPTags([]string{"", "   ", "\t"}))
	require.Equal(t, []string{"a", "b"}, normalizeIPTags([]string{" a ", "a", "b"}))

	long := strings.Repeat("tag", 30) // 90 runes, over VARCHAR(50)
	normalized := normalizeIPTags([]string{long})
	require.Len(t, normalized, 1)
	require.Len(t, []rune(normalized[0]), iptagMaxLen, "single tag must be capped at the ip_tags.tag column length")

	multibyte := strings.Repeat("幻", 60) // 60 CJK runes, 180 bytes
	normalized = normalizeIPTags([]string{multibyte})
	require.Len(t, normalized, 1)
	require.Len(t, []rune(normalized[0]), iptagMaxLen, "truncation counts runes (Postgres CHAR length), not bytes")
}

func setupIPServiceDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.IPTag{}); err != nil {
		t.Fatalf("migrate ip service models: %v", err)
	}
	return db
}

func seedIPServiceUser(t *testing.T, db *gorm.DB, id int64, username string) model.User {
	t.Helper()
	user := model.User{ID: id, Email: username + "@example.test", Username: username, PasswordHash: "hash", Role: "user", Reputation: 10}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}
