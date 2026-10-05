package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/pkg/response"
)

// pathID parses a numeric path parameter under the platform-wide INVALID_ID
// contract（#806 A1，照 pageQuery 配方：helper + path_param_gate_test 守门）：
// base-10 int64 且 > 0 才通过；失败时写 400 INVALID_ID 信封并返回 ok=false，
// 调用方直接 return。
//
// message 为 "" 时走 response.CodeOnly 的 code-only 信封（follow/message
// 历史点位，#669 wire 兼容约定：仅 {"code"}，勿扩字段）；否则走标准
// response.Error 形。
func pathID(c *gin.Context, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		if message == "" {
			response.CodeOnly(c, http.StatusBadRequest, "INVALID_ID")
			return 0, false
		}
		response.Error(c, http.StatusBadRequest, "INVALID_ID", message)
		return 0, false
	}
	return id, true
}
