package service

import (
	"context"
	"errors"
	"log/slog"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/pkg/aliyun"
)

// blockedTextResult is the block predicate for text-facing gates (DM,
// comment, avatar): an explicit "violation" is treated the same as "block".
func blockedTextResult(result string) bool {
	return result == "block" || result == "violation"
}

// blockedImageResult is the block predicate for the sync image gate (feedback
// attachments): only an explicit "block" rejects, mirroring the historical
// image-scan semantics where "review" and "violation" do not block submission.
func blockedImageResult(result string) bool {
	return result == "block"
}

// RunModerationGate is the single A4 moderation gate: it applies the
// environment availability policy to one content-review attempt and records
// the decision with consistent structured log keys (action, env_mode, policy,
// reason). Every moderation gate in the platform routes through this helper so
// release-fail-closed / dev-fail-open semantics stay uniform.
//
// review is nil when no review service is wired; the gate then records
// reason=review_service_not_wired and fails closed in release mode. A review
// error wrapping aliyun.ErrGreenNotConfigured in a non-release environment
// fails open and records reason=green_not_configured; any other review error
// fails closed regardless of environment. subjectNoun/targetNoun render the
// gate-specific log messages (e.g. "content moderation" / "message").
//
// A "block"-matching result returns blockedErr; allow paths return nil.
func RunModerationGate(
	ctx context.Context,
	cfg *config.Config,
	action, subjectNoun, targetNoun string,
	review func(ctx context.Context) (string, error),
	blocked func(result string) bool,
	blockedErr, unavailableErr error,
) error {
	envMode := "unknown"
	release := false
	if cfg != nil {
		envMode = cfg.Server.Mode
		release = cfg.Server.Mode == "release"
	}

	failClosed := func(reason string) error {
		slog.Error(subjectNoun+" unavailable, rejecting "+targetNoun,
			"action", action, "env_mode", envMode, "policy", "fail_closed", "reason", reason)
		return unavailableErr
	}
	failOpen := func(reason string) error {
		slog.Warn(subjectNoun+" skipped, "+targetNoun+" allowed",
			"action", action, "env_mode", envMode, "policy", "fail_open", "reason", reason)
		return nil
	}

	if review == nil {
		if release {
			return failClosed("review_service_not_wired")
		}
		return failOpen("review_service_not_wired")
	}

	result, err := review(ctx)
	if err != nil {
		if !release && errors.Is(err, aliyun.ErrGreenNotConfigured) {
			return failOpen("green_not_configured")
		}
		return failClosed(err.Error())
	}

	if blocked(result) {
		return blockedErr
	}
	return nil
}