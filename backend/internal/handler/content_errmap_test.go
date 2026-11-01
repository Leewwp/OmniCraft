package handler

// content 端点（CreateContent / DownloadContent）的 sentinel→HTTP 契约
// 表（#794 pilot 迁移到 content.go 后随 handler 文件共存）。期望值全部
// 为手写字面量（不从实现表自动生成）：每条规则断言 status 与完整 JSON
// 信封（含固定 message 或 SafeErrorResponse 按 code 解析的安全文案）。
// 另覆盖 fmt.Errorf %w 包装、多 sentinel 组合错误按表序（=迁移前分支
// 序）首命中、两端点各自的未知错误兜底（发布 INTERNAL_ERROR / 下载
// DB_ERROR）与 message 不泄露内部原因。

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/internal/pkg/archivezip"
	"omnicraft/backend/internal/service"
)

// TestPublishErrMap_RuleContract pins every CreateContent rule (order =
// pre-#794 branch order) with handwritten expectations; the wrapped variant
// proves errors.Is reaches the same rule through fmt.Errorf %w chains.
func TestPublishErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "publish frozen", err: service.ErrPublishFrozen, status: http.StatusForbidden, code: "PUBLISH_FROZEN", message: "publishing is temporarily frozen"},
		{name: "source not allowed for original", err: service.ErrSourceNotAllowedForOriginal, status: http.StatusBadRequest, code: "SOURCE_NOT_ALLOWED_FOR_ORIGINAL", message: "original content cannot carry a source attribution"},
		{name: "fanwork source required", err: service.ErrFanworkSourceRequired, status: http.StatusBadRequest, code: "FANWORK_SOURCE_REQUIRED", message: "fanwork content must specify an IP or an inspiration source"},
		{name: "multiple source conflict", err: service.ErrMultipleSourceConflict, status: http.StatusBadRequest, code: "MULTIPLE_SOURCE_CONFLICT", message: "only one of source_original_id and source_fanwork_id may be set"},
		{name: "source original unavailable", err: service.ErrSourceOriginalUnavailable, status: http.StatusBadRequest, code: "SOURCE_ORIGINAL_UNAVAILABLE", message: "source original content does not exist or is unavailable"},
		{name: "source fanwork unavailable", err: service.ErrSourceFanworkUnavailable, status: http.StatusBadRequest, code: "SOURCE_FANWORK_UNAVAILABLE", message: "source fanwork content does not exist or is unavailable"},
		// SafeErrorResponse 按 code 解析；UPLOAD_GRANT_INVALID 不在安全文
		// 案表内，落通用文案（历史行为，迁移前后一致）。
		{name: "upload grant invalid", err: service.ErrUploadGrantInvalid, status: http.StatusBadRequest, code: "UPLOAD_GRANT_INVALID", message: "an unexpected error occurred, please try again later"},
		{name: "media set invalid", err: service.ErrMediaSetInvalid, status: http.StatusBadRequest, code: "MEDIA_SET_INVALID", message: "media set violates the gallery contract"},
		{name: "archive attachment required", err: service.ErrArchiveAttachmentRequired, status: http.StatusBadRequest, code: "ARCHIVE_ATTACHMENT_REQUIRED", message: "mod content requires a zip archive attachment"},
		{name: "attachment policy required", err: service.ErrAttachmentPolicyRequired, status: http.StatusBadRequest, code: "ATTACHMENT_POLICY_REQUIRED", message: "content type requires at least one attachment of the required family"},
		{name: "archive encrypted", err: archivezip.ErrEncrypted, status: http.StatusBadRequest, code: "ARCHIVE_ENCRYPTED", message: "archive is encrypted"},
		{name: "archive path invalid", err: archivezip.ErrPathInvalid, status: http.StatusBadRequest, code: "ARCHIVE_PATH_INVALID", message: "archive path is invalid"},
		{name: "archive link forbidden", err: archivezip.ErrLinkForbidden, status: http.StatusBadRequest, code: "ARCHIVE_LINK_FORBIDDEN", message: "archive link is forbidden"},
		{name: "archive limit exceeded", err: archivezip.ErrLimitExceeded, status: http.StatusBadRequest, code: "ARCHIVE_LIMIT_EXCEEDED", message: "archive limits exceeded"},
		{name: "archive invalid", err: archivezip.ErrInvalid, status: http.StatusBadRequest, code: "ARCHIVE_INVALID", message: "archive is invalid"},
		{name: "archive scan unavailable", err: service.ErrArchiveScanUnavailable, status: http.StatusServiceUnavailable, code: "ARCHIVE_SCAN_UNAVAILABLE", message: "archive scanning is unavailable"},
		{name: "archive scan failed", err: service.ErrArchiveScanFailed, status: http.StatusConflict, code: "ARCHIVE_SCAN_FAILED", message: "archive scan failed"},
		{name: "archive scan pending", err: service.ErrArchiveScanPending, status: http.StatusConflict, code: "ARCHIVE_SCAN_PENDING", message: "archive scan is pending"},
		{name: "upload grant unavailable", err: service.ErrUploadGrantUnavailable, status: http.StatusServiceUnavailable, code: "UPLOAD_GRANT_UNAVAILABLE", message: "an unexpected error occurred, please try again later"},
	}
	if len(cases) != 19 {
		t.Fatalf("publish contract table must keep the historical 19 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, publishErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("publish content: transaction failed: %w", tt.err)
			assertErrMapResponse(t, publishErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestDownloadErrMap_RuleContract pins every DownloadContent rule (order =
// pre-#794 branch order). The wrapped variant proves errors.Is reaches the
// same rule through fmt.Errorf %w chains.
func TestDownloadErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "download unauthorized", err: service.ErrDownloadUnauthorized, status: http.StatusUnauthorized, code: "UNAUTHORIZED", message: "login required"},
		{name: "content not found", err: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{name: "download not published", err: service.ErrDownloadNotPublished, status: http.StatusForbidden, code: "FORBIDDEN", message: "content not available for download"},
		{name: "download unavailable", err: service.ErrDownloadUnavailable, status: http.StatusForbidden, code: "CONTENT_UNAVAILABLE", message: "content is unavailable"},
		{name: "download not allowed", err: service.ErrDownloadNotAllowed, status: http.StatusForbidden, code: "FORBIDDEN", message: "download not allowed"},
		{name: "oss not configured", err: service.ErrOSSNotConfigured, status: http.StatusServiceUnavailable, code: "OSS_NOT_CONFIGURED", message: "oss service not configured"},
		{name: "no attachments", err: service.ErrNoAttachments, status: http.StatusNotFound, code: "NO_ATTACHMENTS", message: "no downloadable files"},
		{name: "invalid attachment id", err: service.ErrInvalidAttachmentID, status: http.StatusBadRequest, code: "INVALID_ATTACHMENT_ID", message: "invalid attachment_id"},
		{name: "attachment mismatch", err: service.ErrAttachmentMismatch, status: http.StatusBadRequest, code: "ATTACHMENT_MISMATCH", message: "attachment does not belong to this content"},
		{name: "ambiguous attachment", err: service.ErrAmbiguousAttachment, status: http.StatusBadRequest, code: "AMBIGUOUS_ATTACHMENT", message: "specify attachment_id; cannot determine a unique primary attachment"},
		{name: "archive not clean", err: service.ErrArchiveNotClean, status: http.StatusForbidden, code: "ARCHIVE_NOT_CLEAN", message: "archive is not clean"},
		{name: "download presign failed", err: service.ErrDownloadPresignFailed, status: http.StatusInternalServerError, code: "OSS_ERROR", message: "failed to generate download url"},
	}
	if len(cases) != 12 {
		t.Fatalf("download contract table must keep the historical 12 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, downloadErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("request download: orchestration failed: %w", tt.err)
			assertErrMapResponse(t, downloadErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestContentErrMaps_MultiMatchErrorKeepsRuleOrder：组合错误同时命中多
// 条规则时，按表序（=迁移前分支序）首个生效——包装文本中的顺序不参与
// 决策。
func TestContentErrMaps_MultiMatchErrorKeepsRuleOrder(t *testing.T) {
	publish := fmt.Errorf("combined failure: %w; %w", service.ErrMediaSetInvalid, service.ErrFanworkSourceRequired)
	assertErrMapResponse(t, publishErrMap, publish, http.StatusBadRequest,
		envelope("FANWORK_SOURCE_REQUIRED", "fanwork content must specify an IP or an inspiration source"))

	download := fmt.Errorf("combined failure: %w; %w", service.ErrAttachmentMismatch, service.ErrContentNotFound)
	assertErrMapResponse(t, downloadErrMap, download, http.StatusNotFound,
		envelope("NOT_FOUND", "content not found"))
}

// TestContentErrMaps_UnknownErrorFallbacks：未知错误（无 sentinel 命中）
// 各端点各自落历史兜底——发布 500 INTERNAL_ERROR、下载 500 DB_ERROR，
// 不能统一；且安全文案不泄露内部原因（dsn/主机/表名/密钥样例均不得出
// 现）。
func TestContentErrMaps_UnknownErrorFallbacks(t *testing.T) {
	leaky := fmt.Errorf("sql: no such table: content_items; dsn=postgres://omnicraft:secret@10.0.0.1:5432/db")

	assertErrMapResponse(t, publishErrMap, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
	assertErrMapResponse(t, downloadErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))

	for _, m := range []struct {
		name string
		err  endpointErrMap
	}{
		{name: "publish", err: publishErrMap},
		{name: "download", err: downloadErrMap},
	} {
		gin.SetMode(gin.TestMode)
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		m.err.write(c, leaky)
		for _, internal := range []string{"secret", "10.0.0.1", "dsn", "no such table", "postgres://"} {
			if strings.Contains(rec.Body.String(), internal) {
				t.Fatalf("%s fallback leaked internal detail %q: %s", m.name, internal, rec.Body.String())
			}
		}
	}
}

// TestContentPresignErrMap_RuleContract pins GenerateOSSToken 的动态校验
// 分流（errors.As UploadValidationError）之后的 sentinel 面（#924 收
// 口）：OSS 未配置 → Safe 503 OSS_NOT_CONFIGURED；其余 Safe 500
// INTERNAL_ERROR。
func TestContentPresignErrMap_RuleContract(t *testing.T) {
	if len(contentPresignErrMap.rules) != 1 {
		t.Fatalf("content presign map must keep exactly 1 rule, got %d", len(contentPresignErrMap.rules))
	}
	assertErrMapResponse(t, contentPresignErrMap, service.ErrOSSNotConfigured, http.StatusServiceUnavailable,
		envelope("OSS_NOT_CONFIGURED", "file storage is not configured"))
	wrapped := fmt.Errorf("presign upload: %w", service.ErrOSSNotConfigured)
	assertErrMapResponse(t, contentPresignErrMap, wrapped, http.StatusServiceUnavailable,
		envelope("OSS_NOT_CONFIGURED", "file storage is not configured"))

	leaky := fmt.Errorf("oss svc: dial tcp 10.0.0.1:5432: secret")
	assertErrMapResponse(t, contentPresignErrMap, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
}

// TestContentDetailErrMap_RuleContract pins GetContent / ListRelated
// Fanworks 共用合同（#924 收口；两路由迁移前一个 `==` 直比、一个
// errors.Is，映射面一致）：NotFound → 404 固定文案；其余 Safe 500
// DB_ERROR。
func TestContentDetailErrMap_RuleContract(t *testing.T) {
	if len(contentDetailErrMap.rules) != 1 {
		t.Fatalf("content detail map must keep exactly 1 rule, got %d", len(contentDetailErrMap.rules))
	}
	assertErrMapResponse(t, contentDetailErrMap, service.ErrContentNotFound, http.StatusNotFound,
		envelope("NOT_FOUND", "content not found"))
	wrapped := fmt.Errorf("get visible content: %w", service.ErrContentNotFound)
	assertErrMapResponse(t, contentDetailErrMap, wrapped, http.StatusNotFound,
		envelope("NOT_FOUND", "content not found"))

	leaky := fmt.Errorf("content repo: sql: no such table %s", "content_items; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, contentDetailErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestUpdateContentErrMap_RuleContract pins UpdateContent（#924 收口
// `==` 直比四分支）：NotFound → 404；非作者 → 403；banned 禁改 → 403
// CONTENT_BANNED；封面非平台对象 → 400 VALIDATION_ERROR（迁移前
// ValidationError 形态，固定文案逐字节保留）；其余 Safe 500 DB_ERROR。
func TestUpdateContentErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "content not found", err: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{name: "content forbidden", err: service.ErrContentForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "not content author"},
		{name: "content banned", err: service.ErrContentBanned, status: http.StatusForbidden, code: "CONTENT_BANNED", message: "content is banned and cannot be edited"},
		{name: "cover not platform oss object", err: service.ErrCoverNotPlatformOSSObject, status: http.StatusBadRequest, code: "VALIDATION_ERROR", message: "cover image must be a platform OSS object"},
	}
	if len(cases) != 4 {
		t.Fatalf("update content contract table must keep the historical 4 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, updateContentErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("update content: %w", tt.err)
			assertErrMapResponse(t, updateContentErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}

	leaky := fmt.Errorf("content svc: sql: no such table %s", "content_items; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, updateContentErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}

// TestDeleteContentErrMap_RuleContract pins DeleteContent（#924 收口
// `==` 直比两分支）：NotFound → 404；非作者 → 403；其余 Safe 500
// DB_ERROR。
func TestDeleteContentErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "content not found", err: service.ErrContentNotFound, status: http.StatusNotFound, code: "NOT_FOUND", message: "content not found"},
		{name: "content forbidden", err: service.ErrContentForbidden, status: http.StatusForbidden, code: "FORBIDDEN", message: "not content author"},
	}
	if len(cases) != 2 {
		t.Fatalf("delete content contract table must keep the historical 2 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, deleteContentErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("delete content: %w", tt.err)
			assertErrMapResponse(t, deleteContentErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}

	leaky := fmt.Errorf("content svc: sql: no such table %s", "content_items; dsn=postgres://u:secret@10.0.0.1/db")
	assertErrMapResponse(t, deleteContentErrMap, leaky, http.StatusInternalServerError,
		envelope("DB_ERROR", "database operation failed, please try again later"))
}
