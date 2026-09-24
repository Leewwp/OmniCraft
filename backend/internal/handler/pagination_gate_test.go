package handler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// #668 守门（source-contract，先例 routes_test.go / judge_wiring_test.go）：
// 标准列表端点的 page/page_size 解析只许走 pagination.go 的 pageQuery——
// handler 非测试源码中出现裸 Query("page") / DefaultQuery("page") 解析即失败。
// 白名单 = 登记例外（独立契约，理由见 pagination.go 注释）：search.go、
// agent.go、admin_trace.go 与 parsePositiveInt 家族（browse_history.go、
// collection.go、content.go 相关内容）。

var rawPageParsePattern = regexp.MustCompile(`(Query|DefaultQuery)\("page"`)

var paginationParseWhitelist = map[string]string{
	"pagination.go":     "pageQuery/clampPage 契约定义处（解析本体）",
	"search.go":         "搜索翻页上限契约（clampSearchPage）",
	"agent.go":          "会话列表可选 page 契约",
	"admin_trace.go":    "admin trace 显式校验（400 快败）",
	"browse_history.go": "parsePositiveInt 0/limit 历史契约",
	"collection.go":     "parseCollectionPagination 钳上界契约",
	"content.go":        "相关内容 parsePositiveInt limit 契约",
}

func handlerSourceFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read handler dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	return files
}

func TestStandardListsHaveNoRawPageParse(t *testing.T) {
	for _, file := range handlerSourceFiles(t) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if _, whitelisted := paginationParseWhitelist[file]; whitelisted {
			// 登记例外：保留独立契约（理由见 pagination.go 注释与本表）。
			continue
		}
		if rawPageParsePattern.Match(raw) {
			lines := []string{}
			for i, line := range strings.Split(string(raw), "\n") {
				if rawPageParsePattern.MatchString(line) {
					lines = append(lines, filepath.Base(file)+":"+itoa(i+1))
				}
			}
			t.Fatalf("%s 出现裸 page 解析（标准列表须走 pageQuery）：\n%s\n若确属独立契约，先在 pagination.go 注释与本测试白名单登记理由",
				file, strings.Join(lines, "\n"))
		}
	}
}

func TestPaginationGateCatchesViolations(t *testing.T) {
	// 反例：新增裸解析必须被守门拦下（谓词有效性证明）。
	violation := []byte(`func listX(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	_ = page
}`)
	if !rawPageParsePattern.Match(violation) {
		t.Fatal("gate pattern must match a new bare DefaultQuery(\"page\") parse")
	}
	clean := []byte(`func listY(c *gin.Context) {
	page, pageSize := pageQuery(c, 20)
	_, _ = page, pageSize
}`)
	if rawPageParsePattern.Match(clean) {
		t.Fatal("gate pattern must not match pageQuery usage")
	}
	// 白名单例外文件模式本身可被识别。
	if _, ok := paginationParseWhitelist["search.go"]; !ok {
		t.Fatal("declared exception search.go must be in the whitelist")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
