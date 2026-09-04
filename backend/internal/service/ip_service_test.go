package service

import (
	"context"
	"errors"
	"fmt"
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

// T15 (F-102): the server must cap tag count at the same limit the client
// enforces (10) — the client-side cap alone lets arbitrary tag counts reach
// ip_tags and the public display path.
func TestNormalizeIPTagsCapsAtTenEntries(t *testing.T) {
	input := make([]string, 0, 14)
	for i := 1; i <= 14; i++ {
		input = append(input, fmt.Sprintf("tag%02d", i))
	}

	got := normalizeIPTags(input)

	require.Len(t, got, 10, "normalizeIPTags must cap the tag list at 10 entries")
	require.Equal(t, []string{"tag01", "tag02", "tag03", "tag04", "tag05", "tag06", "tag07", "tag08", "tag09", "tag10"},
		got, "cap keeps the first-seen order and drops the overflow")
}

// T15 (F-101): tags are public free text and must enter the Green text review
// channel alongside name/description instead of skipping review entirely.
func TestIPReviewInputCarriesTags(t *testing.T) {
	ip := &model.IP{
		ID:          11,
		Name:        "示例 IP",
		Slug:        "example-ip",
		Description: "desc",
		Tags:        []string{"奇幻", "冒险"},
	}
	in := ipReviewInput(ip, 42)

	require.Equal(t, []string{"奇幻", "冒险"}, in.Tags, "IP tags must enter the text review input")
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

// #320: 纯中文（非拉丁）名清洗后 slug 坍缩为 ip-<字节数>——同字节长度的不同名
// 共享同一 slug；旧兜底只试一次 slug-creatorID 且不复查，同 creator 第二个
// 同字数名撞唯一索引直接 500。不同名必须得到不同 slug。
func TestGenerateSlugDistinctForSameByteLengthCJKNames(t *testing.T) {
	a := generateSlug("海贼王")
	b := generateSlug("火影忍")
	if a == b {
		t.Fatalf("same byte-length CJK names must not collapse to one slug: %q", a)
	}
	for _, s := range []string{a, b} {
		if s == "" {
			t.Fatal("slug must not be empty")
		}
	}
}

// #320: 同一 creator 连续创建多个同字节长度中文名 IP 不得因 slug 冲突失败。
func TestCreateIPSameCreatorRepeatedCJKLengthsAllSucceed(t *testing.T) {
	db := setupIPServiceDB(t)
	creator := seedIPServiceUser(t, db, 2, "ip-cjk-creator")
	svc := NewIPService(repository.NewIPRepository(db))

	slugs := map[string]bool{}
	for _, name := range []string{"海贼王", "火影忍", "死神录"} {
		ip, err := svc.CreateIP(context.Background(), CreateIPInput{
			Name:     name,
			Category: "game",
		}, creator.ID)
		if err != nil {
			t.Fatalf("create %s: %v (slug collision 500, #320)", name, err)
		}
		if slugs[ip.Slug] {
			t.Fatalf("duplicate slug %q across distinct names", ip.Slug)
		}
		slugs[ip.Slug] = true
	}
}

// #320: 同名反复创建耗尽全部兜底变体后返回 ErrIPSlugTaken（handler 映射 409），
// 而不是撞唯一索引冒 500。
func TestCreateIPSlugExhaustionReturnsSlugTaken(t *testing.T) {
	db := setupIPServiceDB(t)
	creator := seedIPServiceUser(t, db, 3, "ip-exhaust-creator")
	svc := NewIPService(repository.NewIPRepository(db))

	for i := 0; i < 9; i++ { // base + creator 后缀 + 2..8 号尾缀 = 9 个可用变体
		if _, err := svc.CreateIP(context.Background(), CreateIPInput{Name: "同名字库", Category: "game"}, creator.ID); err != nil {
			t.Fatalf("create #%d should succeed: %v", i+1, err)
		}
	}
	if _, err := svc.CreateIP(context.Background(), CreateIPInput{Name: "同名字库", Category: "game"}, creator.ID); !errors.Is(err, ErrIPSlugTaken) {
		t.Fatalf("exhausted create should return ErrIPSlugTaken, got %v", err)
	}
}
