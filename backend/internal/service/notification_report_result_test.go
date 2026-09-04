package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
)

// T53（FIX-28a）：NotifyReportResolution helper 契约——admin 处理举报后通知
// 举报者（channel=system、type=report_result、target=report、body 带处置说明）。
func TestNotifyReportResolutionContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Notification{}))

	producer := &t11CaptureProducer{}
	svc := newT11NotificationService(db, producer)

	svc.NotifyReportResolution(42, 1001, "resolved", "已删除违规内容", 7)
	svc.NotifyReportResolution(43, 1002, "dismissed", "经核实不构成违规", 7)
	svc.NotifyReportResolution(44, 1003, "resolved", "", 7)

	var notifies []map[string]interface{}
	for _, p := range producer.publishes {
		if p.Topic == "notification.create" && p.Payload["type"] == "report_result" {
			notifies = append(notifies, p.Payload)
		}
	}
	require.Len(t, notifies, 3, "resolved/dismissed/空处置说明各一条")

	first := notifies[0]
	require.Equal(t, float64(42), first["user_id"])
	require.Equal(t, "system", first["channel"])
	require.Equal(t, "report", first["target_type"])
	require.Equal(t, float64(1001), first["target_id"])
	require.Contains(t, first["body"], "已删除违规内容")

	second := notifies[1]
	require.Contains(t, second["body"], "经核实不构成违规")

	third := notifies[2]
	require.NotEmpty(t, third["body"], "空处置说明时使用兜底文案，不得发出空 body")
}
