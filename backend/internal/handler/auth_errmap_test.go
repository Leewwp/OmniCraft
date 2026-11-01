package handler

// auth 端点的 sentinel→HTTP 契约表（#923 从手写 if 链迁到 endpointErrMap
// 声明）。期望值全部为手写字面量（=迁移前 if 分支的逐字节 wire）；五份
// map 各一张表，规则数锁历史值防漂移。Login 的 recordLoginFailure 记账
// 副作用是纯副作用分支，留在 handler 内（分支体无响应调用、门合法），
// 副作用先于 write() 的顺序由既有端点测试锁行为（登录失败计数见
// TestLoginRequiresCaptchaAfterFailureThreshold），不属 map 契约面。

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"omnicraft/backend/internal/service"
)

// TestRegisterErrMap_RuleContract pins every Register rule (order =
// pre-#923 branch order).
func TestRegisterErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "user already exists", err: service.ErrUserAlreadyExists, status: http.StatusConflict, code: "USER_EXISTS", message: "email already registered"},
		{name: "username taken", err: service.ErrUsernameTaken, status: http.StatusConflict, code: "USERNAME_TAKEN", message: "username already taken"},
	}
	if len(cases) != 2 {
		t.Fatalf("register contract table must keep the historical 2 rules, got %d", len(cases))
	}
	if len(registerErrMap.rules) != 2 {
		t.Fatalf("register map must keep exactly 2 rules, got %d", len(registerErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, registerErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("register pending: repository failure: %w", tt.err)
			assertErrMapResponse(t, registerErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestLoginErrMap_RuleContract pins every Login rule (order = pre-#923
// branch order; ErrInvalidCredentials 分支迁移前的 recordLoginFailure 记
// 账副作用保留在 handler 的纯副作用分支内，先于本 map 的 write）。
func TestLoginErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invalid credentials", err: service.ErrInvalidCredentials, status: http.StatusUnauthorized, code: "INVALID_CREDENTIALS", message: "invalid email or password"},
		{name: "user banned", err: service.ErrUserBanned, status: http.StatusForbidden, code: service.DenialReasonUserBanned, message: "account has been banned"},
		{name: "email not verified", err: service.ErrEmailNotVerified, status: http.StatusForbidden, code: service.DenialReasonEmailNotVerified, message: "email verification required before login"},
	}
	if len(cases) != 3 {
		t.Fatalf("login contract table must keep the historical 3 rules, got %d", len(cases))
	}
	if len(loginErrMap.rules) != 3 {
		t.Fatalf("login map must keep exactly 3 rules, got %d", len(loginErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, loginErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("login: credential check failed: %w", tt.err)
			assertErrMapResponse(t, loginErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestResetPasswordErrMap_RuleContract pins every ResetPassword rule (order
// = pre-#923 branch order).
func TestResetPasswordErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invalid token", err: service.ErrInvalidToken, status: http.StatusBadRequest, code: "INVALID_TOKEN", message: "invalid or expired reset token"},
		{name: "password too short", err: service.ErrPasswordTooShort, status: http.StatusBadRequest, code: "PASSWORD_TOO_SHORT", message: "password does not meet minimum length requirement"},
		{name: "password too long", err: service.ErrPasswordTooLong, status: http.StatusBadRequest, code: "PASSWORD_TOO_LONG", message: "password exceeds the 72-byte maximum length"},
	}
	if len(cases) != 3 {
		t.Fatalf("reset password contract table must keep the historical 3 rules, got %d", len(cases))
	}
	if len(resetPasswordErrMap.rules) != 3 {
		t.Fatalf("reset password map must keep exactly 3 rules, got %d", len(resetPasswordErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, resetPasswordErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("reset password: verification failure: %w", tt.err)
			assertErrMapResponse(t, resetPasswordErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestVerifyEmailErrMap_RuleContract pins the VerifyEmail rule. 注意其
// INVALID_TOKEN 文案与 ResetPassword 的同 code 文案不同——两份 map 分开声
// 明的原因。
func TestVerifyEmailErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "invalid token", err: service.ErrInvalidToken, status: http.StatusBadRequest, code: "INVALID_TOKEN", message: "invalid or expired verification token"},
	}
	if len(cases) != 1 {
		t.Fatalf("verify email contract table must keep the historical 1 rule, got %d", len(cases))
	}
	if len(verifyEmailErrMap.rules) != 1 {
		t.Fatalf("verify email map must keep exactly 1 rule, got %d", len(verifyEmailErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, verifyEmailErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("verify email: token consumed: %w", tt.err)
			assertErrMapResponse(t, verifyEmailErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestResendVerificationErrMap_RuleContract pins the ResendVerification
// rule（pending 与 legacy 两个调用点共享同一合同）。
func TestResendVerificationErrMap_RuleContract(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{name: "resend cooldown", err: service.ErrResendCooldown, status: http.StatusTooManyRequests, code: "RESEND_COOLDOWN", message: "please wait before requesting another verification email"},
	}
	if len(cases) != 1 {
		t.Fatalf("resend verification contract table must keep the historical 1 rule, got %d", len(cases))
	}
	if len(resendVerificationErrMap.rules) != 1 {
		t.Fatalf("resend verification map must keep exactly 1 rule, got %d", len(resendVerificationErrMap.rules))
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			assertErrMapResponse(t, resendVerificationErrMap, tt.err, tt.status, envelope(tt.code, tt.message))
			wrapped := fmt.Errorf("resend verification: cooldown window: %w", tt.err)
			assertErrMapResponse(t, resendVerificationErrMap, wrapped, tt.status, envelope(tt.code, tt.message))
		})
	}
}

// TestAuthErrMap_FallbackContract：五份 map 的未知错误兜底保留历史固定
// 文案（fallbackMessage），且不泄露内部错误细节（迁移前兜底分支无日志
// 副作用，故无 fallbackLog）。
func TestAuthErrMap_FallbackContract(t *testing.T) {
	maps := []struct {
		name        string
		m           endpointErrMap
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "register", m: registerErrMap, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMessage: "failed to register user"},
		{name: "login", m: loginErrMap, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMessage: "failed to login"},
		{name: "reset password", m: resetPasswordErrMap, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMessage: "failed to reset password"},
		{name: "verify email", m: verifyEmailErrMap, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMessage: "failed to verify email"},
		{name: "resend verification", m: resendVerificationErrMap, wantStatus: http.StatusInternalServerError, wantCode: "INTERNAL_ERROR", wantMessage: "failed to send verification email"},
	}
	for _, tt := range maps {
		t.Run(tt.name, func(t *testing.T) {
			boom := errors.New("auth service: dial tcp 10.9.9.9:5432: boom")
			assertErrMapResponse(t, tt.m, boom, tt.wantStatus, envelope(tt.wantCode, tt.wantMessage))

			body := lastErrMapBody(t, tt.m, boom)
			for _, internal := range []string{"dial tcp", "10.9.9.9", "auth service"} {
				if strings.Contains(body, internal) {
					t.Fatalf("fallback leaked internal detail %q: %s", internal, body)
				}
			}
		})
	}
}
