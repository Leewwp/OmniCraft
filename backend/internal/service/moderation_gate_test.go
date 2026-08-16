package service

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
)

func captureGateLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)
	fn()
	return buf.String()
}

// TestRunModerationGateA4Semantics pins the single moderationGate helper's
// A4 environment availability policy: release fails closed on any review
// failure, non-release only fails open for aliyun.ErrGreenNotConfigured, and
// every decision is recorded with consistent structured log keys.
func TestRunModerationGateA4Semantics(t *testing.T) {
	const action = "test_action"
	release := &config.Config{Server: config.ServerConfig{Mode: "release"}}
	debug := &config.Config{Server: config.ServerConfig{Mode: "debug"}}

	errReview := func(err error) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { return "", err }
	}
	resultReview := func(result string) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { return result, nil }
	}

	tests := []struct {
		name           string
		cfg            *config.Config
		review         func(context.Context) (string, error)
		blockViolation bool
		blockedErr     error
		unavailableErr error
		wantErr        error
		wantLog        []string
	}{
		{
			name:           "unwired reviewer and release fails closed",
			cfg:            release,
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_closed", "reason=review_service_not_wired", "env_mode=release", "action=" + action},
		},
		{
			name:           "unwired reviewer and debug fails open",
			cfg:            debug,
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_open", "reason=review_service_not_wired", "env_mode=debug", "action=" + action},
		},
		{
			name:           "unwired reviewer and nil cfg fails open with unknown env",
			cfg:            nil,
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_open", "reason=review_service_not_wired", "env_mode=unknown", "action=" + action},
		},
		{
			name:           "review error and release fails closed with raw reason",
			cfg:            release,
			review:         errReview(errors.New("green api down")),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_closed", "env_mode=release", "action=" + action, "reason="},
		},
		{
			name:           "non-green review error and debug still fails closed",
			cfg:            debug,
			review:         errReview(errors.New("green api down")),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_closed", "env_mode=debug", "action=" + action, "reason="},
		},
		{
			name:           "green not configured and release fails closed",
			cfg:            release,
			review:         errReview(aliyun.ErrGreenNotConfigured),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_closed", "env_mode=release", "action=" + action},
		},
		{
			name:           "green not configured and debug fails open",
			cfg:            debug,
			review:         errReview(aliyun.ErrGreenNotConfigured),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantLog:        []string{"policy=fail_open", "reason=green_not_configured", "env_mode=debug", "action=" + action},
		},
		{
			name:           "block result rejected",
			cfg:            debug,
			review:         resultReview("block"),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrTextBlocked,
		},
		{
			name:           "violation result rejected by text gate",
			cfg:            debug,
			review:         resultReview("violation"),
			blockViolation: true,
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
			wantErr:        ErrTextBlocked,
		},
		{
			name:           "violation result allowed by image gate",
			cfg:            debug,
			review:         resultReview("violation"),
			blockedErr:     ErrFeedbackAttachmentBlocked,
			unavailableErr: ErrFeedbackAttachmentModerationUnavailable,
		},
		{
			name:           "review result allowed",
			cfg:            debug,
			review:         resultReview("review"),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
		},
		{
			name:           "pass result allowed",
			cfg:            debug,
			review:         resultReview("pass"),
			blockedErr:     ErrTextBlocked,
			unavailableErr: ErrModerationUnavailable,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logged := captureGateLogs(t, func() {
				err := RunModerationGate(context.Background(), tc.cfg, action,
					"content moderation", "submission", tc.review, tc.blockViolation,
					tc.blockedErr, tc.unavailableErr)
				if tc.wantErr != nil {
					if !errors.Is(err, tc.wantErr) {
						t.Fatalf("error = %v, want %v", err, tc.wantErr)
					}
				} else if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
			})
			for _, frag := range tc.wantLog {
				if !strings.Contains(logged, frag) {
					t.Fatalf("logs %q do not contain %q", logged, frag)
				}
			}
			if tc.wantErr == nil && len(tc.wantLog) == 0 && strings.Contains(logged, "policy=") {
				t.Fatalf("allow path must not log a policy decision: %q", logged)
			}
		})
	}
}
