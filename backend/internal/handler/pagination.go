package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// SP-25 FR-10（低-26）：平台统一列表分页钳制契约。
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// clampPage normalizes page/page_size to the platform-wide list contract:
// page ≥ 1; 1 ≤ page_size ≤ 100, out-of-range (or unparseable → 0) page_size
// falls back to defaultPageSize. Every STANDARD list endpoint clamps through
// this helper so `page_size=100000` can never dump a full table (#668 收口：
// 标准列表裸 page 解析由 pagination_gate_test.go 守门）。
//
// 登记例外（独立契约，非标准列表钳制，勿迁 pageQuery）：
//   - search.go：clampSearchPage + maxSearchPage（config 驱动搜索翻页上限，
//     成本门契约）；
//   - agent.go：会话列表可选 page（缺省=近期全量，语义不同于标准列表）；
//   - admin_trace.go：非法 page → 400 快败（显式校验契约，非静默归一）；
//   - browse_history.go / collection.go / content.go 相关内容：parsePositiveInt
//     家族（0/limit 双参数或钳上界 100 而非回落 20 的历史契约）。
func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}
	return page, pageSize
}

// pageQuery parses the standard page/page_size query params with the given
// default page size (non-positive → 20) and clamps them via clampPage.
func pageQuery(c *gin.Context, defaultSize int) (page, pageSize int) {
	if defaultSize <= 0 {
		defaultSize = defaultPageSize
	}
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultSize)))
	return clampPage(page, pageSize)
}
