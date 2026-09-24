package response

import (
	"github.com/gin-gonic/gin"
)

// #669 错误信封收口的 wire 兼容窄 helper。
//
// 历史点位中存在两类不走 {code,message} 标准形的错误响应，为保真 wire
// 契约由本包窄 helper 承接（迁移账本：docs/working/2026-09-25-669-envelope-census.md）：
//   - code-only ×10：仅 {"code"}，勿扩字段；
//   - captcha_result ×4：附加 captcha_result:false（captcha.go），不塞 details。
//
// 与 Error 一致使用 AbortWithStatusJSON（#669 核对：非测试代码无 IsAborted
// 消费方，中止语义变更无行为面）。

// CodeOnly sends the historical code-only error envelope: {"code": "..."}.
func CodeOnly(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code})
}

// CaptchaError sends the captcha error envelope with the extra
// captcha_result:false field (never folded into details).
func CaptchaError(c *gin.Context, status int, code string, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"code":           code,
		"message":        message,
		"captcha_result": false,
	})
}
