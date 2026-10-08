package repository

import (
	"testing"

	"omnicraft/backend/internal/model"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #846 仓储层合同：内容三读路径（FindByID / ListContents / BatchGetByIDs）
// 挂载关联 IP 摘要 {id, name, cover_url}——原创行不挂（zone 语义）、悬空
// ip_id 落 nil、摘要不携带 IP 行的其余字段（slug/status 等）。

func setupIPSummaryDB(t *testing.T) (*ContentRepository, *model.IP, *model.ContentItem, *model.ContentItem, *model.ContentItem) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.IP{}, &model.ContentItem{}))

	author := model.User{Email: "a846-author@example.test", Username: "a846-author", PasswordHash: "hash", Reputation: 10}
	require.NoError(t, db.Create(&author).Error)

	ip := model.IP{
		Name:     "星穹铁道",
		Slug:     "a846-ip",
		CoverURL: "https://oss.example.com/ips/a846-cover.jpg",
		Status:   "approved",
	}
	require.NoError(t, db.Create(&ip).Error)

	danglingID := int64(999999)
	fanwork := model.ContentItem{
		Title:       "a846 fanwork",
		AuthorID:    author.ID,
		Zone:        "fanwork",
		IPID:        &ip.ID,
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
	}
	original := model.ContentItem{
		Title:       "a846 original",
		AuthorID:    author.ID,
		Zone:        "original",
		ContentType: "article",
		Status:      "published",
		IsPublic:    true,
	}
	dangling := model.ContentItem{
		Title:       "a846 dangling ip",
		AuthorID:    author.ID,
		Zone:        "fanwork",
		IPID:        &danglingID,
		ContentType: "image",
		Status:      "published",
		IsPublic:    true,
	}
	require.NoError(t, db.Create(&fanwork).Error)
	require.NoError(t, db.Create(&original).Error)
	require.NoError(t, db.Create(&dangling).Error)

	return NewContentRepository(db), &ip, &fanwork, &original, &dangling
}

func requireIPSummary(t *testing.T, got *model.IP, want *model.IP) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.CoverURL, got.CoverURL)
	// 摘要只取 {id, name, cover_url}：IP 行其余列不得随内容响应外泄。
	require.Empty(t, got.Slug)
	require.Empty(t, got.Status)
	require.Nil(t, got.Creator)
}

func TestFindByIDHydratesIPSummary(t *testing.T) {
	repo, ip, fanwork, original, dangling := setupIPSummaryDB(t)

	got, err := repo.FindByID(fanwork.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	requireIPSummary(t, got.IP, ip)

	// 原创内容不挂 IP（zone 判断语义保持，前端 original 无 IP 展示）。
	gotOriginal, err := repo.FindByID(original.ID)
	require.NoError(t, err)
	require.NotNil(t, gotOriginal)
	require.Nil(t, gotOriginal.IP)

	// 悬空 ip_id（IP 行已不存在）降级为 nil，不得报错。
	gotDangling, err := repo.FindByID(dangling.ID)
	require.NoError(t, err)
	require.NotNil(t, gotDangling)
	require.Nil(t, gotDangling.IP)
}

func TestListContentsHydratesIPSummary(t *testing.T) {
	repo, ip, fanwork, _, dangling := setupIPSummaryDB(t)

	items, total, err := repo.ListContents(ListContentsFilter{Zone: "fanwork"})
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, items, 2)

	byID := make(map[int64]model.ContentItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	requireIPSummary(t, byID[fanwork.ID].IP, ip)
	require.Nil(t, byID[dangling.ID].IP)
}

func TestBatchGetByIDsHydratesIPSummary(t *testing.T) {
	repo, ip, fanwork, original, dangling := setupIPSummaryDB(t)

	items, err := repo.BatchGetByIDs([]int64{fanwork.ID, original.ID, dangling.ID})
	require.NoError(t, err)
	require.Len(t, items, 3)

	byID := make(map[int64]model.ContentItem, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	requireIPSummary(t, byID[fanwork.ID].IP, ip)
	require.Nil(t, byID[original.ID].IP)
	require.Nil(t, byID[dangling.ID].IP)
}
