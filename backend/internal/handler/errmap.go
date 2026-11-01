package handler

// errmap 是 handler 层 sentinel→HTTP 映射的常规路径（#794 pilot，C3 推广）：
// 每个端点在自己的 handler 文件内声明 endpointErrMap（规则集 + 兜底引用
// 共享命名预设），本文件只拥有类型、写法语义与预设集——映射知识留在
// handler module（业务 module 不依赖 HTTP 概念）。匹配语义与手写映射链
// 逐字一致：按声明顺序 errors.Is，首个命中生效——不做 map[error] 直接
// 查找（漏 wrapped error），不遍历无序 map（丢顺序）。
//
// 惯用法（新端点照此声明；回归门 errmap_gate_test.go 拦手写链回潮）：
//
//  1. 无状态映射声明为包级 var，规则集与兜底在一份声明里可读：
//
//	var xxxErrMap = endpointErrMap{
//		rules: []errorRule{
//			{sentinel: service.ErrX, status: http.StatusBadRequest, code: "X_INVALID"},
//		},
//		fallback: fallbackInternal,
//	}
//
//  2. 兜底需要固定安全文案或兜底日志时，在预设字段上逐字节保留迁移前
//     的固定 message 与 slog 文案：
//
//	fallback: errFallback{
//		status:  fallbackInternal.status,
//		code:    fallbackInternal.code,
//		message: "Failed to submit feedback",
//		log:     "failed to submit feedback",
//	},
//
//  3. 规则命中需要 handler 状态耦合的响应前副作用（如按分支变化的审计
//     元数据）时，map 声明为 handler 构造器内建的实例字段（闭包捕获
//     h），规则上挂 before；包级 var map 保持无状态。纯副作用分支（命
//     中后只记账不发响应，随后统一 write() 发响应）留在 handler 内即
//     可，不需要任何 hook。
//
// 两种 message 语义（固定文案 / SafeErrorResponse 按 code 取安全文案）
// 都禁止裸 err.Error() 进响应。

import (
	"errors"
	"log/slog"
	"net/http"

	"omnicraft/backend/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// errorRule 是一条 sentinel→HTTP 响应映射。message 非空 = 固定安全文
// 案，经 response.Error 发送（迁移前的固定 message 分支）；message 为
// 空 = 沿用 response.SafeErrorResponse 按 code 取安全文案（迁移前的
// Safe 分支）。before 非空时在发送响应前执行（副作用在前、响应在后，
// 与迁移前手写分支逐行同序），仅限实例级 map 使用。
type errorRule struct {
	sentinel error
	status   int
	code     string
	message  string
	before   func(c *gin.Context, err error)
}

// errFallback 是单个端点的未知错误兜底：status/code 恒定；message 非空
// = 固定安全文案（response.Error），空 = SafeErrorResponse 按 code 取安
// 全文案；log 非空 = 兜底响应前 slog.Error(log, "error", err)，承接迁
// 移前兜底分支的日志副作用。
type errFallback struct {
	status  int
	code    string
	message string
	log     string
}

// 共享命名预设：预设名即文档。端点兜底直接引用预设，或在预设字段上扩
// 展 message/log；不各自发明兜底组合。
var (
	// fallbackInternal：500 INTERNAL_ERROR + Safe 通用文案——「未知错误
	// 视作端点自身故障」的历史兜底，多数端点用它。
	fallbackInternal = errFallback{status: http.StatusInternalServerError, code: "INTERNAL_ERROR"}
	// fallbackDB：500 DB_ERROR + Safe 存储层文案——「未知错误视作存储
	// 故障」的历史兜底（content 下载、social 面等）。
	fallbackDB = errFallback{status: http.StatusInternalServerError, code: "DB_ERROR"}
)

// endpointErrMap 是单个端点持有的完整错误合同：有序规则集 + 未知错误
// 兜底（引用共享预设）。无状态 map 声明为包级 var；状态耦合 map 声明
// 为 handler 构造器的实例字段。
type endpointErrMap struct {
	rules    []errorRule
	fallback errFallback
}

// write 将 err 映射为 HTTP 响应：按顺序 errors.Is 匹配规则，首个命中先
// 执行规则级 before（如有）再发送该规则的响应；无命中先落兜底日志（如
// 有）再发送本端点兜底。副作用一律先于响应。
func (m endpointErrMap) write(c *gin.Context, err error) {
	for _, rule := range m.rules {
		if errors.Is(err, rule.sentinel) {
			if rule.before != nil {
				rule.before(c, err)
			}
			if rule.message != "" {
				response.Error(c, rule.status, rule.code, rule.message)
			} else {
				response.SafeErrorResponse(c, rule.status, rule.code, err)
			}
			return
		}
	}
	if m.fallback.log != "" {
		slog.Error(m.fallback.log, "error", err)
	}
	if m.fallback.message != "" {
		response.Error(c, m.fallback.status, m.fallback.code, m.fallback.message)
	} else {
		response.SafeErrorResponse(c, m.fallback.status, m.fallback.code, err)
	}
}
