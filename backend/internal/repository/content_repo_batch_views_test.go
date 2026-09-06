package repository

import (
	"fmt"
	"regexp"
	"testing"

	"omnicraft/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// #400 子项 1（TDD）：BatchIncrViewCounts 生成的 UPDATE 语句把 CASE 表达式
// 直接赋给 view_count 列。历史 bug：caseStmt 自带 "view_count = " 前缀，
// GORM 再拼一层列赋值 → SET view_count = view_count = CASE ...（内层 = 是
// boolean 比较）→ PostgreSQL SQLSTATE 42804（bigint 列收到 boolean）。
// sqlite 不报错（宽松类型），因此以 dry-run 捕获生成 SQL 断言形态。
func TestBatchIncrViewCountsGeneratesDirectCaseAssignment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentItem{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var captured []string
	session := db.Session(&gorm.Session{DryRun: true, Logger: logger.New(
		&captureWriter{lines: &captured}, logger.Config{LogLevel: logger.Info},
	)})
	repo := NewContentRepository(session)
	if err := repo.BatchIncrViewCounts(map[int64]int64{7: 2, 9: 1}); err != nil {
		t.Fatalf("batch incr: %v", err)
	}

	joined := ""
	for _, line := range captured {
		joined += line + "\n"
	}
	doubleAssign := regexp.MustCompile("`view_count`?=\\s*view_count\\s*=")
	directAssign := regexp.MustCompile("`view_count`?=\\s*CASE\\s+`?id`?\\s+WHEN")
	if doubleAssign.MatchString(joined) {
		t.Fatalf("generated SQL assigns a boolean comparison to view_count (PG 42804):\n%s", joined)
	}
	if !directAssign.MatchString(joined) {
		t.Fatalf("generated SQL must assign the CASE expression directly:\n%s", joined)
	}
}

type captureWriter struct{ lines *[]string }

func (w *captureWriter) Printf(format string, args ...interface{}) {
	*w.lines = append(*w.lines, fmt.Sprintf(format, args...))
}
