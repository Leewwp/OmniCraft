package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

// T52（FIX-23b）：我的 IP——创建者可查本人全部状态的 IP（含 pending/rejected），
// rejected 行携带最新一次拒绝原因；仅本人可见（其他创建者的 IP 不出现）。

func setupMyIPsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.IP{}, &model.IPTag{}, &model.IPReviewLog{}); err != nil {
		t.Fatalf("migrate models: %v", err)
	}
	return db
}

func TestListMyIPsReturnsAllStatusesOnlyForOwner(t *testing.T) {
	db := setupMyIPsDB(t)
	svc := NewIPService(repository.NewIPRepository(db))

	creator := seedIPServiceUser(t, db, 1, "myip-owner")
	other := seedIPServiceUser(t, db, 2, "myip-other")

	mine := []string{"pending", "approved", "rejected"}
	for i, status := range mine {
		ip := &model.IP{ID: int64(10 + i), Name: "My IP " + status, Slug: "my-ip-" + status, Category: "game", CreatorID: &creator.ID, Status: status}
		require.NoError(t, db.Create(ip).Error)
	}
	// 其他创建者的同名/同状态 IP 不得出现
	foreign := &model.IP{ID: 20, Name: "Foreign IP", Slug: "foreign-ip", Category: "game", CreatorID: &other.ID, Status: "pending"}
	require.NoError(t, db.Create(foreign).Error)

	ips, total, err := svc.ListMyIPs(context.Background(), creator.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 3, total, "本人全部状态 IP 共 3 条（含 pending/rejected）")
	require.Len(t, ips, 3)
	for _, ip := range ips {
		require.Equal(t, "My IP "+ip.Status, ip.Name)
	}
}

func TestListMyIPsCarriesLatestRejectReason(t *testing.T) {
	db := setupMyIPsDB(t)
	svc := NewIPService(repository.NewIPRepository(db))

	creator := seedIPServiceUser(t, db, 1, "myip-reason-owner")
	ip := &model.IP{ID: 30, Name: "Rejected IP", Slug: "rejected-ip", Category: "game", CreatorID: &creator.ID, Status: "rejected"}
	require.NoError(t, db.Create(ip).Error)
	require.NoError(t, db.Create(&model.IPReviewLog{IPID: ip.ID, Action: "reject", Reason: "第一次原因"}).Error)
	require.NoError(t, db.Create(&model.IPReviewLog{IPID: ip.ID, Action: "reject", Reason: "最新拒绝原因"}).Error)

	ips, _, err := svc.ListMyIPs(context.Background(), creator.ID, 1, 20)
	require.NoError(t, err)
	require.Len(t, ips, 1)
	require.Equal(t, "最新拒绝原因", ips[0].ReviewReason, "必须携带最新一次拒绝原因（T16 落库）")
}
