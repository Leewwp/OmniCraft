package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
)

// run-1 审计 #6：IP proposal 写路径（CreateProposal/SubmitVote）此前绕过
// #446 的 visibleIPForViewer 闸——不可见（pending/rejected）IP 上可建提案、
// 可投票直至 adoptTx 改写隐藏 IP 的 profile。修复后写路径与读路径同口径。

type proposalVisibilityStack struct {
	svc      *IPProposalService
	db       *gorm.DB
	creator  model.User
	follower model.User
	ip       *model.IP
}

func setupProposalVisibilityStack(t *testing.T) *proposalVisibilityStack {
	t.Helper()
	svc, db := setupProposalService(t)
	creator := &model.User{Email: "vis-creator@example.com", Username: "viscreator", PasswordHash: "x", Reputation: 10, Role: "user"}
	follower := &model.User{Email: "vis-follower@example.com", Username: "visfollower", PasswordHash: "x", Reputation: 10, Role: "user"}
	require.NoError(t, db.Create(creator).Error)
	require.NoError(t, db.Create(follower).Error)

	ip := &model.IP{
		Name: "隐藏 IP", Slug: "hidden-ip", Description: "旧简介",
		Status: "pending", CreatorID: &creator.ID,
	}
	require.NoError(t, db.Create(ip).Error)
	require.NoError(t, db.Create(&model.Follow{
		FollowerID: follower.ID, TargetType: "ip", TargetID: ip.ID,
	}).Error)

	desc := "隐藏 IP 上的提案"
	require.NoError(t, db.Create(&model.IPProposal{
		IPID: ip.ID, ProposerID: creator.ID, Status: "open",
		DescriptionChange: &desc, ModerationState: "approved",
		DeadlineAt: time.Now().Add(48 * time.Hour),
	}).Error)

	return &proposalVisibilityStack{svc: svc, db: db, creator: *creator, follower: *follower, ip: ip}
}

func TestCreateProposalOnInvisibleIPRejectedForNonCreator(t *testing.T) {
	stack := setupProposalVisibilityStack(t)

	desc := "非创建者在 pending IP 上建提案"
	_, err := stack.svc.CreateProposal(t.Context(), stack.ip.ID, stack.follower.ID, CreateIPProposalInput{DescriptionChange: &desc})
	require.ErrorIs(t, err, ErrIPNotFound, "non-creator proposal on a pending ip must be rejected with IP-not-found semantics")

	// 创建者保留 studio 自助路径：可见性闸放行后继续走既有业务校验
	//（种子已挂开放提案 → 此处应命中 ErrProposalOpenExists 而非可见性拒绝）。
	_, err = stack.svc.CreateProposal(t.Context(), stack.ip.ID, stack.creator.ID, CreateIPProposalInput{DescriptionChange: &desc})
	require.ErrorIs(t, err, ErrProposalOpenExists, "creator passes the visibility gate and hits the ordinary open-proposal guard instead")
	require.NotErrorIs(t, err, ErrIPNotFound)
}

func TestSubmitVoteOnInvisibleIPRejected(t *testing.T) {
	stack := setupProposalVisibilityStack(t)

	var proposal model.IPProposal
	require.NoError(t, stack.db.Where("ip_id = ?", stack.ip.ID).First(&proposal).Error)

	err := stack.svc.SubmitVote(t.Context(), proposal.ID, stack.follower.ID, "yes")
	require.ErrorIs(t, err, ErrIPNotFound, "voting on a proposal of a non-approved ip must be rejected for non-creators")
}
