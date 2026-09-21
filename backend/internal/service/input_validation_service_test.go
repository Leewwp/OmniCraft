package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"omnicraft/backend/config"
)

// SP-25 FR-11 service 层契约。

// 低-21：未知审核结论转人工（此前 default→pass 是 fail-open）。
func TestNormalizeReviewResultUnknownFailsToManualReview(t *testing.T) {
	require.Equal(t, "review", NormalizeReviewResult("totally-unknown-verdict"))
	require.Equal(t, "review", NormalizeReviewResult("  WHATEVER  "))
	require.Equal(t, "block", NormalizeReviewResult("violation"))
	require.Equal(t, "block", NormalizeReviewResult("BLOCK"))
	require.Equal(t, "review", NormalizeReviewResult("pending"))
	require.Equal(t, "pass", NormalizeReviewResult("pass"))
	require.Equal(t, "pass", NormalizeReviewResult("approved"))
	// 空值维持 pass 语义（未送审内容直通），仅未知值收紧转人工。
	require.Equal(t, "pass", NormalizeReviewResult(""))
}

// 低-19：视频上传 duration_sec 必填，省略即拒而非跳过上限校验。
func TestVideoUploadRequiresDuration(t *testing.T) {
	s := &OSSService{cfg: &config.Config{}}
	s.cfg.Limits.VideoMaxMB = 512
	s.cfg.Limits.VideoMaxSec = 600
	s.cfg.Limits.VideoMaxMB = 512

	dur := func(v int) *int { return &v }
	err := s.validateUploadByType("video", "video/mp4", 1024, nil, ".mp4")
	require.Error(t, err, "missing duration must be rejected")
	require.Contains(t, err.Error(), "duration_sec is required")

	err = s.validateUploadByType("video", "video/mp4", 1024, dur(0), ".mp4")
	require.Error(t, err, "zero duration must be rejected")

	err = s.validateUploadByType("video", "video/mp4", 1024, dur(3600), ".mp4")
	require.Error(t, err, "over-limit duration must be rejected")

	err = s.validateUploadByType("video", "video/mp4", 1024, dur(60), ".mp4")
	require.NoError(t, err, "in-range duration passes")
}

// 低-22：提案封面平台域（提交端）——外域图被拒、平台域放行。
func TestProposalCoverPlatformDomain(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	svc.cfg.OSS.Domain = "https://cdn.example-oss.test"

	external := "https://evil.example.com/cover.png"
	_, err := svc.CreateProposal(t.Context(), ip.ID, proposer.ID, CreateIPProposalInput{CoverURLChange: &external})
	require.ErrorIs(t, err, ErrProposalCoverNotPlatform)

	platform := "https://cdn.example-oss.test/covers/x.png"
	_, err = svc.CreateProposal(t.Context(), ip.ID, proposer.ID, CreateIPProposalInput{CoverURLChange: &platform})
	require.NotErrorIs(t, err, ErrProposalCoverNotPlatform, "platform-domain cover must pass the domain gate")
}

// 低-25：改密 72 字节语义——rune 预算与 bcrypt 字节预算存在缺口，handler
// 侧的 byte 检查是该缺口的唯一闭环（128 CJK runes = 384 bytes）。
func TestPasswordByteBudgetExceedsRuneBudget(t *testing.T) {
	pw := strings.Repeat("长", 128)
	require.Equal(t, 128, len([]rune(pw)))
	require.Greater(t, len(pw), 72, "rune-capped binding alone cannot enforce the bcrypt byte budget")
}
