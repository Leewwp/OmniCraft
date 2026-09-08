package handler

// #411 F3：GET /stats/summary 的 zone 参数契约——
// 合法值（original|fanwork）按分区统计返回 200；非法值 400 INVALID_PARAM；
// 缺省保持全局语义向后兼容。

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/service"
)

func setupStatsHandlerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&model.IP{}, &model.ContentItem{}, &model.User{}))

	// fanwork 1 条（作者 u1）/ original 2 条（作者 u2）/ pending 1 条不计。
	require.NoError(t, db.Create(&model.User{Email: "u1@t.local", Username: "u1", PasswordHash: "x"}).Error)
	require.NoError(t, db.Create(&model.User{Email: "u2@t.local", Username: "u2", PasswordHash: "x"}).Error)
	require.NoError(t, db.Create(&model.ContentItem{Title: "f1", Zone: "fanwork", Status: "published", AuthorID: 1}).Error)
	require.NoError(t, db.Create(&model.ContentItem{Title: "o1", Zone: "original", Status: "published", AuthorID: 2}).Error)
	require.NoError(t, db.Create(&model.ContentItem{Title: "o2", Zone: "original", Status: "published", AuthorID: 2}).Error)
	require.NoError(t, db.Create(&model.ContentItem{Title: "pend", Zone: "fanwork", Status: "pending", AuthorID: 1}).Error)

	svc := service.NewStatsService(db, nil)
	// 预热校验 service 行为符合本测试的期望值设定。
	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(3), summary.Contents)

	r := gin.New()
	r.GET("/api/v1/stats/summary", NewStatsHandler(svc).GetSummary)
	return r
}

func TestStatsHandlerZoneParamContract(t *testing.T) {
	r := setupStatsHandlerTest(t)

	cases := []struct {
		name     string
		query    string
		status   int
		contents int64
		users    int64
	}{
		{"no zone keeps global semantics", "", http.StatusOK, 3, 2},
		{"fanwork zone filters contents and counts distinct authors", "zone=fanwork", http.StatusOK, 1, 1},
		{"original zone filters contents and counts distinct authors", "zone=original", http.StatusOK, 2, 1},
		{"invalid zone rejected", "zone=all", http.StatusBadRequest, 0, 0},
		{"case-sensitive vocabulary", "zone=Fanwork", http.StatusBadRequest, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/summary?"+tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code)
			if tc.status != http.StatusOK {
				return
			}
			var body struct {
				Summary service.StatsSummary `json:"summary"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			require.Equal(t, tc.contents, body.Summary.Contents)
			require.Equal(t, tc.users, body.Summary.Users)
		})
	}
}
