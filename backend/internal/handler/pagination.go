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
// falls back to defaultPageSize. Every list endpoint clamps through this
// helper so `page_size=100000` can never dump a full table.
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
