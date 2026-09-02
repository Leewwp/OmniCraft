package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

func setupProposalService(t *testing.T) (*IPProposalService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	// 单连接让 :memory: 库在事务（新连接）间共享同一份数据（既有测试同法）
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{}, &model.IP{}, &model.IPTag{}, &model.Follow{},
		&model.IPProposal{}, &model.IPProposalVote{}, &model.IPProfileVersion{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cfg := &config.Config{
		Reputation: config.ReputationConfig{MinScoreForInteraction: 3},
		IPProposal: config.IPProposalConfig{MinVotes: 3, PassThreshold: 0.6, DeadlineDays: 7},
	}

	svc := NewIPProposalService(
		repository.NewIPRepository(db),
		repository.NewUserRepository(db),
		repository.NewFollowRepository(db),
		repository.NewIPProposalRepository(db),
		nil, // no redis in tests: cache invalidation is a no-op
		cfg,
	)
	return svc, db
}

func seedProposalFixtures(t *testing.T, db *gorm.DB) (proposer, follower, outsider *model.User, ip *model.IP) {
	t.Helper()
	proposer = &model.User{Email: "prop-proposer@example.com", Username: "proposer", PasswordHash: "x", Reputation: 10, Role: "user"}
	follower = &model.User{Email: "prop-follower@example.com", Username: "follower", PasswordHash: "x", Reputation: 10, Role: "user"}
	outsider = &model.User{Email: "prop-outsider@example.com", Username: "outsider", PasswordHash: "x", Reputation: 10, Role: "user"}
	for _, u := range []*model.User{proposer, follower, outsider} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	ip = &model.IP{Name: "Proposal IP", Slug: "proposal-ip", Description: "旧简介", Status: "published", CreatorID: &proposer.ID}
	if err := db.Create(ip).Error; err != nil {
		t.Fatalf("seed ip: %v", err)
	}
	for _, tag := range []string{"科幻", "太空"} {
		if err := db.Create(&model.IPTag{IPID: ip.ID, Tag: tag}).Error; err != nil {
			t.Fatalf("seed tag: %v", err)
		}
	}
	if err := db.Create(&model.Follow{FollowerID: follower.ID, TargetType: "ip", TargetID: ip.ID}).Error; err != nil {
		t.Fatalf("seed follow: %v", err)
	}
	// 发起人参与共治投票同样需要先关注该 IP（投票权=关注+信誉分，提案者不豁免）
	if err := db.Create(&model.Follow{FollowerID: proposer.ID, TargetType: "ip", TargetID: ip.ID}).Error; err != nil {
		t.Fatalf("seed proposer follow: %v", err)
	}
	return proposer, follower, outsider, ip
}

func strPtr(s string) *string { return &s }

func TestCreateProposalValidatesFieldsAndReputation(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	cases := []struct {
		name    string
		userID  int64
		input   CreateIPProposalInput
		wantErr error
	}{
		{
			name:   "description happy path",
			userID: proposer.ID,
			input:  CreateIPProposalInput{DescriptionChange: strPtr("新简介：一个太空歌剧世界观")},
		},
		{
			name:    "empty proposal rejected",
			userID:  proposer.ID,
			input:   CreateIPProposalInput{},
			wantErr: ErrProposalEmpty,
		},
		{
			name:    "add existing tag rejected",
			userID:  proposer.ID,
			input:   CreateIPProposalInput{TagsAdd: []string{"科幻"}},
			wantErr: ErrProposalTagConflict,
		},
		{
			name:    "remove missing tag rejected",
			userID:  proposer.ID,
			input:   CreateIPProposalInput{TagsRemove: []string{"不存在"}},
			wantErr: ErrProposalTagConflict,
		},
		{
			name:    "tag add and remove overlap rejected",
			userID:  proposer.ID,
			input:   CreateIPProposalInput{TagsAdd: []string{"新标签"}, TagsRemove: []string{"新标签"}},
			wantErr: ErrProposalTagConflict,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposal, err := svc.CreateProposal(ctx, ip.ID, tc.userID, tc.input)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got proposal %+v", tc.wantErr, proposal)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if proposal.Status != "open" {
				t.Fatalf("status = %s, want open", proposal.Status)
			}
			if time.Until(proposal.DeadlineAt) <= 0 {
				t.Fatalf("deadline %v must be in the future", proposal.DeadlineAt)
			}
		})
	}
}

func TestCreateProposalRejectsLowReputationAndDuplicateOpen(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)

	lowRep := &model.User{Email: "prop-lowrep@example.com", Username: "lowrep", PasswordHash: "x", Reputation: 1, Role: "user"}
	if err := db.Create(lowRep).Error; err != nil {
		t.Fatalf("seed lowrep: %v", err)
	}

	if _, err := svc.CreateProposal(t.Context(), ip.ID, lowRep.ID, CreateIPProposalInput{DescriptionChange: strPtr("x")}); err != ErrProposalNotEligible {
		t.Fatalf("low reputation want ErrProposalNotEligible, got %v", err)
	}

	if _, err := svc.CreateProposal(t.Context(), ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("第一案")}); err != nil {
		t.Fatalf("first proposal: %v", err)
	}
	if _, err := svc.CreateProposal(t.Context(), ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("第二案")}); err != ErrProposalOpenExists {
		t.Fatalf("second open proposal want ErrProposalOpenExists, got %v", err)
	}
}

func TestVoteRequiresFollowerAndIsImmutable(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, follower, outsider, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	proposal, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("改简介")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 未关注者不可投票
	if err := svc.SubmitVote(ctx, proposal.ID, outsider.ID, "yes"); err != ErrProposalNotEligible {
		t.Fatalf("outsider vote want ErrProposalNotEligible, got %v", err)
	}
	// 提案人本人可投且计入
	if err := svc.SubmitVote(ctx, proposal.ID, proposer.ID, "yes"); err != nil {
		t.Fatalf("proposer vote: %v", err)
	}
	// 一人一票，投后不可改
	if err := svc.SubmitVote(ctx, proposal.ID, proposer.ID, "no"); err != ErrProposalAlreadyVoted {
		t.Fatalf("double vote want ErrProposalAlreadyVoted, got %v", err)
	}
	// 关注者投反对
	if err := svc.SubmitVote(ctx, proposal.ID, follower.ID, "no"); err != nil {
		t.Fatalf("follower vote: %v", err)
	}

	got, err := svc.GetProposal(ctx, proposal.ID, proposer.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.YesVotes != 1 || got.NoVotes != 1 {
		t.Fatalf("counts = %d/%d, want 1/1", got.YesVotes, got.NoVotes)
	}
	if got.MyVote == nil || *got.MyVote != "yes" {
		t.Fatalf("my_vote = %v, want yes", got.MyVote)
	}
}

func TestProposalAdoptsAtThresholdAndAppliesProfile(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	// min_votes=3 / threshold=0.6 → 需要 3 赞成（3/3=1.0 ≥0.6；2 赞 1 反=0.67 也过，但不足 3 票）
	proposal, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{
		DescriptionChange: strPtr("新简介"),
		TagsAdd:           []string{"歌剧"},
		TagsRemove:        []string{"太空"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	voters := make([]*model.User, 0, 3)
	for i := 0; i < 2; i++ {
		u := &model.User{Email: "prop-voter-" + string(rune('a'+i)) + "@example.com", Username: "voter" + string(rune('a'+i)), PasswordHash: "x", Reputation: 5, Role: "user"}
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("seed voter: %v", err)
		}
		if err := db.Create(&model.Follow{FollowerID: u.ID, TargetType: "ip", TargetID: ip.ID}).Error; err != nil {
			t.Fatalf("seed follow: %v", err)
		}
		voters = append(voters, u)
	}
	for _, u := range voters {
		if err := svc.SubmitVote(ctx, proposal.ID, u.ID, "yes"); err != nil {
			t.Fatalf("vote: %v", err)
		}
	}
	// 2 票：未达 min_votes=3，提案仍 open
	open, err := svc.GetProposal(ctx, proposal.ID, proposer.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if open.Status != "open" {
		t.Fatalf("status = %s before threshold, want open", open.Status)
	}

	// 第 3 票（提案人赞成）→ 3/3 ≥ 门槛 → adopted 并生效
	if err := svc.SubmitVote(ctx, proposal.ID, proposer.ID, "yes"); err != nil {
		t.Fatalf("deciding vote: %v", err)
	}

	adopted, err := svc.GetProposal(ctx, proposal.ID, proposer.ID)
	if err != nil {
		t.Fatalf("get adopted: %v", err)
	}
	if adopted.Status != "adopted" || adopted.EffectiveAt == nil {
		t.Fatalf("status=%s effective=%v, want adopted with effective_at", adopted.Status, adopted.EffectiveAt)
	}

	// IP 资料已生效：简介更新、标签加减
	var updated model.IP
	if err := db.First(&updated, ip.ID).Error; err != nil {
		t.Fatalf("reload ip: %v", err)
	}
	if updated.Description != "新简介" {
		t.Fatalf("description = %q, want 新简介", updated.Description)
	}
	var tags []model.IPTag
	db.Where("ip_id = ?", ip.ID).Find(&tags)
	got := map[string]bool{}
	for _, tg := range tags {
		got[tg.Tag] = true
	}
	if !got["歌剧"] || got["太空"] || !got["科幻"] {
		t.Fatalf("tags after adoption = %v, want 科幻+歌剧 without 太空", got)
	}

	// 版本快照含旧值
	var version model.IPProfileVersion
	if err := db.Where("proposal_id = ?", proposal.ID).First(&version).Error; err != nil {
		t.Fatalf("version snapshot missing: %v", err)
	}
	if version.Snapshot == "" || version.Changes == "" {
		t.Fatalf("snapshot/changes must be recorded, got %q/%q", version.Snapshot, version.Changes)
	}
}

func TestProposalRejectsAtDeadlineLazily(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	proposal, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("过期案")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 直接把 deadline 拨到过去，模拟到期
	past := time.Now().Add(-time.Hour)
	if err := db.Model(&model.IPProposal{}).Where("id = ?", proposal.ID).Update("deadline_at", past).Error; err != nil {
		t.Fatalf("age proposal: %v", err)
	}

	got, err := svc.GetProposal(ctx, proposal.ID, proposer.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "rejected" {
		t.Fatalf("status = %s after deadline, want rejected", got.Status)
	}
	// 到期后不可再投票
	if err := svc.SubmitVote(ctx, proposal.ID, proposer.ID, "yes"); err != ErrProposalClosed {
		t.Fatalf("vote on expired want ErrProposalClosed, got %v", err)
	}
}

func TestListProposalsFiltersAndClosesExpired(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	first, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("一")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	open, openTotal, err := svc.ListProposals(ctx, ip.ID, "open", "", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list open: %v", err)
	}
	if len(open) != 1 || open[0].ID != first.ID {
		t.Fatalf("open list = %+v", open)
	}
	if openTotal != 1 {
		t.Fatalf("open total = %d, want 1", openTotal)
	}

	past := time.Now().Add(-time.Hour)
	db.Model(&model.IPProposal{}).Where("id = ?", first.ID).Update("deadline_at", past)

	openAfter, _, err := svc.ListProposals(ctx, ip.ID, "open", "", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list open after expiry: %v", err)
	}
	if len(openAfter) != 0 {
		t.Fatalf("expired proposal must leave open list, got %+v", openAfter)
	}
	rejectedViews, rejectedTotal, err := svc.ListProposals(ctx, ip.ID, "rejected", "", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list rejected: %v", err)
	}
	if len(rejectedViews) != 1 || rejectedTotal != 1 {
		t.Fatalf("rejected list = %+v total=%d", rejectedViews, rejectedTotal)
	}
}

// #290 IP 内搜索：提案列表按 q 过滤（description_change 包含匹配），tab 计数
// 用 status=all 跨状态 total；GovernanceDisplay 暴露门槛供卡片刻度渲染。
func TestListProposalsQueryFiltersAndTotal(t *testing.T) {
	svc, db := setupProposalService(t)
	proposer, _, _, ip := seedProposalFixtures(t, db)
	ctx := t.Context()

	if _, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("一改简介")}); err != nil {
		t.Fatalf("create first: %v", err)
	}
	db.Model(&model.IPProposal{}).Where("ip_id = ?", ip.ID).Update("deadline_at", time.Now().Add(-time.Hour))
	if _, _, err := svc.ListProposals(ctx, ip.ID, "open", "", 0, 0, proposer.ID); err != nil {
		t.Fatalf("close expired: %v", err)
	}
	second, err := svc.CreateProposal(ctx, ip.ID, proposer.ID, CreateIPProposalInput{DescriptionChange: strPtr("二改简介")})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}

	all, allTotal, err := svc.ListProposals(ctx, ip.ID, "all", "", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if allTotal != 2 || len(all) != 2 {
		t.Fatalf("all = %d rows total=%d, want 2/2", len(all), allTotal)
	}

	hit, hitTotal, err := svc.ListProposals(ctx, ip.ID, "all", "二改", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list q: %v", err)
	}
	if hitTotal != 1 || len(hit) != 1 || hit[0].ID != second.ID {
		t.Fatalf("q filter = %+v total=%d, want only second", hit, hitTotal)
	}

	miss, missTotal, err := svc.ListProposals(ctx, ip.ID, "open", "一改", 0, 0, proposer.ID)
	if err != nil {
		t.Fatalf("list open q: %v", err)
	}
	if missTotal != 0 || len(miss) != 0 {
		t.Fatalf("open+q=一改 should be empty, got %+v total=%d", miss, missTotal)
	}

	minVotes, threshold := svc.GovernanceDisplay()
	if minVotes != 3 || threshold != 0.6 {
		t.Fatalf("governance display = %d/%v, want 3/0.6", minVotes, threshold)
	}
}
