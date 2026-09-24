package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"omnicraft/backend/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// #668：主 feed ListContents 迁 pageQuery 后，响应回显的 page/page_size
// 与仓储实际使用的有效参数一致（历史行为：原样回显 0/100000 而仓储按 1/20
// 查询——handler 与仓储参数分叉）。此为可观察 API metadata 修复；查询单页
// 上限由仓储防线（content_repo_pagination_test.go）另行锁住。

func TestListContentsEchoesEffectivePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rows := make([]model.ContentItem, 0, 30)
	for i := 0; i < 30; i++ {
		rows = append(rows, model.ContentItem{AuthorID: 1, Status: "published", Title: "主 feed 项", Zone: "original"})
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	h, _ := newContentHandlerForTest(db, testOSSUploadConfig(), nil)
	r := gin.New()
	r.GET("/contents", h.ListContents)

	cases := []struct {
		name               string
		query              string
		wantPage, wantSize int
	}{
		{"oversize page_size echoes effective 20", "page=1&page_size=100000", 1, 20},
		{"zero/negative page echoes 1", "page=0&page_size=20", 1, 20},
		{"negative page echoes 1", "page=-1&page_size=20", 1, 20},
		{"in-range passthrough", "page=2&page_size=10", 2, 10},
		{"absent params default", "", 1, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/contents?"+tc.query, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
			}
			var payload struct {
				Page     int              `json:"page"`
				PageSize int              `json:"page_size"`
				Contents []map[string]any `json:"contents"`
				Total    int64            `json:"total"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if payload.Page != tc.wantPage || payload.PageSize != tc.wantSize {
				t.Fatalf("echoed page/page_size = %d/%d, want %d/%d (effective params)",
					payload.Page, payload.PageSize, tc.wantPage, tc.wantSize)
			}
			if len(payload.Contents) > tc.wantSize {
				t.Fatalf("single page returned %d rows, exceeding effective page_size %d",
					len(payload.Contents), tc.wantSize)
			}
		})
	}
}
