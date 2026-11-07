package handler

// collab_invite 三路由（SendInvite / AcceptInvite / DeclineInvite）共享
// 一份 collabInviteErrMap（迁移前是共享 switch 函数 mapCollabInviteError，
// 18 case + Safe 兜底）。契约表按 map 声明粒度锁定全部 18 条规则（顺序
// =迁移前 case 顺序，message 为 spec 原文中文文案）；路由级行为由
// collab_invite_test.go 的真库真服务用例覆盖（smoke），此处不再逐路由
// 重复。

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestCollabInviteErrMap_RuleContract pins every collab invite rule (order
// = pre-#922 switch case order); the wrapped variant proves errors.Is
// reaches the same rule through fmt.Errorf %w chains.
func TestCollabInviteErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invite self not allowed", err: service.ErrInviteSelfNotAllowed, status: http.StatusBadRequest, code: "INVITE_SELF_NOT_ALLOWED", message: "不能邀请自己"},
		{name: "invite author not allowed", err: service.ErrInviteAuthorNotAllowed, status: http.StatusBadRequest, code: "INVITE_AUTHOR_NOT_ALLOWED", message: "不能邀请内容作者"},
		{name: "invitee already contributor", err: service.ErrInviteAlreadyContributor, status: http.StatusConflict, code: "INVITE_ALREADY_CONTRIBUTOR", message: "对方已是联合创作者"},
		{name: "invitee unavailable", err: service.ErrInviteeUnavailable, status: http.StatusNotFound, code: "INVITEE_UNAVAILABLE", message: "邀请对象不可用"},
		{name: "content unavailable", err: service.ErrContentUnavailable, status: http.StatusNotFound, code: "CONTENT_UNAVAILABLE", message: "内容不可用"},
		{name: "contributor limit reached", err: service.ErrContributorLimitReached, status: http.StatusConflict, code: "CONTRIBUTOR_LIMIT_REACHED", message: "联合创作者数量已达上限"},
		{name: "not content owner", err: service.ErrNotContentOwner, status: http.StatusForbidden, code: "NOT_CONTENT_OWNER", message: "仅作者或已确认的联合创作者可以邀请"},
		{name: "reputation too low", err: service.ErrReputationTooLow, status: http.StatusForbidden, code: "REPUTATION_TOO_LOW", message: "信誉分不足，无法邀请"},
		{name: "invite daily limit", err: service.ErrInviteDailyLimit, status: http.StatusTooManyRequests, code: "INVITE_DAILY_LIMIT", message: "今日邀请次数已达上限"},
		{name: "invite duplicate user", err: service.ErrInviteDuplicateUser, status: http.StatusConflict, code: "INVITE_DUPLICATE_USER", message: "今日已向该用户发送过邀请"},
		{name: "invite blocked", err: service.ErrInviteBlocked, status: http.StatusForbidden, code: "INVITE_BLOCKED", message: "双方存在拉黑关系"},
		{name: "invitee not accepting", err: service.ErrInviteNotAccepting, status: http.StatusForbidden, code: "INVITE_NOT_ACCEPTING", message: "该用户已关闭联合创作邀请接收"},
		{name: "invite already exists", err: service.ErrInviteAlreadyExists, status: http.StatusConflict, code: "INVITE_ALREADY_EXISTS", message: "已向该用户发送过邀请"},
		{name: "invite rate limit unavailable", err: service.ErrInviteRateLimitUnavailable, status: http.StatusServiceUnavailable, code: "INVITE_SERVICE_UNAVAILABLE", message: "邀请服务暂时不可用"},
		{name: "invite not found", err: service.ErrInviteNotFound, status: http.StatusNotFound, code: "INVITE_NOT_FOUND", message: "邀请不存在"},
		{name: "invite expired", err: service.ErrInviteExpired, status: http.StatusBadRequest, code: "INVITE_EXPIRED", message: "邀请已过期"},
		{name: "not invitee", err: service.ErrInviteNotInvitee, status: http.StatusForbidden, code: "INVITE_NOT_INVITEE", message: "仅受邀用户可以操作该邀请"},
		{name: "invite not pending", err: service.ErrInviteNotPending, status: http.StatusConflict, code: "INVITE_NOT_PENDING", message: "邀请已处理"},
	}
	if len(cases) != 18 {
		t.Fatalf("collab invite contract table must keep the historical 18 rules, got %d", len(cases))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, collabInviteErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("collab invite service: storage failure: %w", tt.err)
			assertErrMapResponse(t, collabInviteErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestCollabInviteErrMap_UnknownFallback：未知错误落历史 Safe 兜底（500
// INTERNAL_ERROR，通用安全文案，不泄露内部原因）——与迁移前
// mapCollabInviteError 的 default 分支逐字节一致。
func TestCollabInviteErrMap_UnknownFallback(t *testing.T) {
	leaky := errors.New("redis: connection refused; addr=10.0.0.2:6379; secret-token")

	assertErrMapResponse(t, collabInviteErrMap, leaky, http.StatusInternalServerError,
		envelope("INTERNAL_ERROR", "an unexpected error occurred, please try again later"))
}
