package handler

// #794 sentinel→HTTP 映射注册表 pilot：CreateContent / DownloadContent 两
// 个入口的 sentinel→status/code/message 合同从 content.go 的散列 if 分支
// 收敛为小型有序规则表，映射知识留在 handler module（业务 module 不依赖
// HTTP 概念）。匹配语义与迁移前逐字一致：按声明顺序 errors.Is，首个命中
// 生效——不做 map[error] 直接查找（漏 wrapped error），不遍历无序 map（丢
// 顺序）。两个端点各持自己的规则集与兜底，互不共用。

import (
	"errors"
	"net/http"

	"omnicraft/backend/internal/pkg/archivezip"
	"omnicraft/backend/internal/pkg/response"
	"omnicraft/backend/internal/service"

	"github.com/gin-gonic/gin"
)

// errorRule 是一条 sentinel→HTTP 响应映射。message 非空 = 固定安全文案，
// 经 response.Error 发送（迁移前的固定 message 分支）；message 为空 = 沿用
// response.SafeErrorResponse 按 code 取安全文案（迁移前的 Safe 分支）。两
// 种发送方式都禁止裸 err.Error() 进响应。
type errorRule struct {
	sentinel error
	status   int
	code     string
	message  string
}

// endpointErrMap 是单个端点持有的完整错误合同：有序规则集 + 未知错误兜
// 底。兜底始终走 SafeErrorResponse，由各端点自己声明（发布与下载的历史
// 兜底不同，不能统一）。
type endpointErrMap struct {
	rules          []errorRule
	fallbackStatus int
	fallbackCode   string
}

// write 将 err 映射为 HTTP 响应：按顺序 errors.Is 匹配规则，首个命中发
// 送该规则的响应；无命中发送本端点兜底。
func (m endpointErrMap) write(c *gin.Context, err error) {
	for _, rule := range m.rules {
		if errors.Is(err, rule.sentinel) {
			if rule.message != "" {
				response.Error(c, rule.status, rule.code, rule.message)
			} else {
				response.SafeErrorResponse(c, rule.status, rule.code, err)
			}
			return
		}
	}
	response.SafeErrorResponse(c, m.fallbackStatus, m.fallbackCode, err)
}

// publishErrMap 是 POST /contents（CreateContent）的 sentinel→HTTP 合同，
// 顺序与迁移前 content.go 分支顺序逐条一致；未知错误兜底保持历史 500
// INTERNAL_ERROR（与下载端点的 DB_ERROR 兜底不同）。
var publishErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrPublishFrozen, status: http.StatusForbidden, code: "PUBLISH_FROZEN"},
		{sentinel: service.ErrSourceNotAllowedForOriginal, status: http.StatusBadRequest, code: "SOURCE_NOT_ALLOWED_FOR_ORIGINAL"},
		{sentinel: service.ErrFanworkSourceRequired, status: http.StatusBadRequest, code: "FANWORK_SOURCE_REQUIRED"},
		{sentinel: service.ErrMultipleSourceConflict, status: http.StatusBadRequest, code: "MULTIPLE_SOURCE_CONFLICT"},
		{sentinel: service.ErrSourceOriginalUnavailable, status: http.StatusBadRequest, code: "SOURCE_ORIGINAL_UNAVAILABLE"},
		{sentinel: service.ErrSourceFanworkUnavailable, status: http.StatusBadRequest, code: "SOURCE_FANWORK_UNAVAILABLE"},
		{sentinel: service.ErrUploadGrantInvalid, status: http.StatusBadRequest, code: "UPLOAD_GRANT_INVALID"},
		{sentinel: service.ErrMediaSetInvalid, status: http.StatusBadRequest, code: "MEDIA_SET_INVALID"},
		{sentinel: service.ErrArchiveAttachmentRequired, status: http.StatusBadRequest, code: "ARCHIVE_ATTACHMENT_REQUIRED", message: "mod content requires a zip archive attachment"},
		// #690：注册表 attachment_policy（required_any_of）违规面——任何
		// 配置了 policy 的类型共用，非 3d_print 专属分支。
		{sentinel: service.ErrAttachmentPolicyRequired, status: http.StatusBadRequest, code: "ATTACHMENT_POLICY_REQUIRED", message: "content type requires at least one attachment of the required family"},
		{sentinel: archivezip.ErrEncrypted, status: http.StatusBadRequest, code: "ARCHIVE_ENCRYPTED", message: "archive is encrypted"},
		{sentinel: archivezip.ErrPathInvalid, status: http.StatusBadRequest, code: "ARCHIVE_PATH_INVALID", message: "archive path is invalid"},
		{sentinel: archivezip.ErrLinkForbidden, status: http.StatusBadRequest, code: "ARCHIVE_LINK_FORBIDDEN", message: "archive link is forbidden"},
		{sentinel: archivezip.ErrLimitExceeded, status: http.StatusBadRequest, code: "ARCHIVE_LIMIT_EXCEEDED", message: "archive limits exceeded"},
		{sentinel: archivezip.ErrInvalid, status: http.StatusBadRequest, code: "ARCHIVE_INVALID", message: "archive is invalid"},
		{sentinel: service.ErrArchiveScanUnavailable, status: http.StatusServiceUnavailable, code: "ARCHIVE_SCAN_UNAVAILABLE", message: "archive scanning is unavailable"},
		{sentinel: service.ErrArchiveScanFailed, status: http.StatusConflict, code: "ARCHIVE_SCAN_FAILED", message: "archive scan failed"},
		{sentinel: service.ErrArchiveScanPending, status: http.StatusConflict, code: "ARCHIVE_SCAN_PENDING", message: "archive scan is pending"},
		{sentinel: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE"},
	},
	fallbackStatus: http.StatusInternalServerError,
	fallbackCode:   "INTERNAL_ERROR",
}

// downloadErrMap 是 GET /contents/:id/download（DownloadContent）的
// sentinel→HTTP 合同，顺序与迁移前 content.go 分支顺序逐条一致；未知错误
// 兜底保持历史 500 DB_ERROR。该合同同时是 SP-16 #451 MCP 工具
// omnicraft_request_download 的 REST 面投影，语义不得分叉。
var downloadErrMap = endpointErrMap{
	rules: []errorRule{
		{sentinel: service.ErrDownloadUnauthorized, status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "login required"},
		{sentinel: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{sentinel: service.ErrDownloadNotPublished, status: http.StatusForbidden, code: "FORBIDDEN", message: "content not available for download"},
		{sentinel: service.ErrDownloadUnavailable, status: http.StatusForbidden, code: "CONTENT_UNAVAILABLE", message: "content is unavailable"},
		{sentinel: service.ErrDownloadNotAllowed, status: http.StatusForbidden, code: "FORBIDDEN", message: "download not allowed"},
		{sentinel: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED", message: "oss service not configured"},
		{sentinel: service.ErrNoAttachments, status: http.StatusNotFound, code: "NO_ATTACHMENTS", message: "no downloadable files"},
		{sentinel: service.ErrInvalidAttachmentID, status: http.StatusBadRequest, code: "INVALID_ATTACHMENT_ID", message: "invalid attachment_id"},
		{sentinel: service.ErrAttachmentMismatch, status: http.StatusBadRequest, code: "ATTACHMENT_MISMATCH", message: "attachment does not belong to this content"},
		{sentinel: service.ErrAmbiguousAttachment, status: http.StatusBadRequest, code: "AMBIGUOUS_ATTACHMENT", message: "specify attachment_id; cannot determine a unique primary attachment"},
		{sentinel: service.ErrArchiveNotClean, status: http.StatusForbidden, code: "ARCHIVE_NOT_CLEAN", message: "archive is not clean"},
		{sentinel: service.ErrDownloadPresignFailed, status: http.StatusInternalServerError, code: "OSS_ERROR", message: "failed to generate download url"},
	},
	fallbackStatus: http.StatusInternalServerError,
	fallbackCode:   "DB_ERROR",
}
