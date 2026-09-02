package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
	"omnicraft/backend/internal/pkg/recovery"
	"omnicraft/backend/internal/repository"
)

var (
	ErrProposalEmpty        = errors.New("proposal must change at least one field")
	ErrProposalTagConflict  = errors.New("tag proposal conflicts with current ip tags")
	ErrProposalOpenExists   = errors.New("an open proposal already exists for this ip")
	ErrProposalNotEligible  = errors.New("user is not eligible for ip proposal voting")
	ErrProposalAlreadyVoted = errors.New("user already voted on this proposal")
	ErrProposalClosed       = errors.New("proposal is closed")
	ErrProposalNotFound     = errors.New("proposal not found")
)

// CreateIPProposalInput is the field-level change set of one proposal.
// Name/slug/category are locked and can never be proposed (#290).
type CreateIPProposalInput struct {
	DescriptionChange *string  `json:"description_change,omitempty"`
	CoverURLChange    *string  `json:"cover_url_change,omitempty"`
	TagsAdd           []string `json:"tags_add,omitempty"`
	TagsRemove        []string `json:"tags_remove,omitempty"`
}

// IPProposalView is the API-facing projection: the proposal row plus the
// viewer's own vote (nil when not voted / anonymous).
type IPProposalView struct {
	model.IPProposal
	MyVote *string `json:"my_vote,omitempty"`
}

type IPProposalService struct {
	ipRepo       *repository.IPRepository
	userRepo     *repository.UserRepository
	followRepo   *repository.FollowRepository
	proposalRepo *repository.IPProposalRepository
	rdb          *redis.Client
	cfg          *config.Config
	reviewSvc    *ReviewService
	notify       func(userID int64, channel, notifType, title, body, targetType string, targetID int64, senderID int64)
}

func NewIPProposalService(
	ipRepo *repository.IPRepository,
	userRepo *repository.UserRepository,
	followRepo *repository.FollowRepository,
	proposalRepo *repository.IPProposalRepository,
	rdb *redis.Client,
	cfg *config.Config,
) *IPProposalService {
	return &IPProposalService{
		ipRepo:       ipRepo,
		userRepo:     userRepo,
		followRepo:   followRepo,
		proposalRepo: proposalRepo,
		rdb:          rdb,
		cfg:          cfg,
	}
}

// SetReviewService wires the AI text moderation pass for proposal text
// (same fail-open path as IP creation, #290 story 23). Without it proposals
// skip the review trail (tests / local no-op).
func (s *IPProposalService) SetReviewService(svc *ReviewService) {
	s.reviewSvc = svc
}

// SetNotifier wires the notification fan-out (notification_service.Notify).
// Without it the proposal domain stays silent (tests / local no-op).
func (s *IPProposalService) SetNotifier(fn func(userID int64, channel, notifType, title, body, targetType string, targetID int64, senderID int64)) {
	s.notify = fn
}

func (s *IPProposalService) fanout(userID int64, notifType, title, body string, targetID, senderID int64) {
	if s.notify == nil || userID <= 0 {
		return
	}
	s.notify(userID, "in_app", notifType, title, body, "ip", targetID, senderID)
}

func (s *IPProposalService) thresholds() (minVotes int, threshold float64, deadlineDays int) {
	c := config.IPProposalConfig{}
	if s.cfg != nil {
		c = s.cfg.IPProposal
	}
	return c.EffectiveMinVotes(), c.EffectivePassThreshold(), c.EffectiveDeadlineDays()
}

func (s *IPProposalService) minReputation() int {
	if s.cfg == nil || s.cfg.Reputation.MinScoreForInteraction <= 0 {
		return 3
	}
	return s.cfg.Reputation.MinScoreForInteraction
}

func (s *IPProposalService) ipTags(ipID int64) (map[string]bool, error) {
	rows, err := s.ipRepo.GetTags(ipID)
	if err != nil {
		return nil, err
	}
	tags := make(map[string]bool, len(rows))
	for _, row := range rows {
		tags[row.Tag] = true
	}
	return tags, nil
}

// CreateProposal validates eligibility and the field-level change set, then
// stores the single open proposal for the IP. The proposal text goes through
// the same async fail-open AI review as IP creation (story 23).
func (s *IPProposalService) CreateProposal(ctx context.Context, ipID, proposerID int64, input CreateIPProposalInput) (*model.IPProposal, error) {
	user, err := s.userRepo.FindByID(proposerID)
	if err != nil || user == nil {
		return nil, ErrProposalNotEligible
	}
	if user.IsBanned || user.Reputation < s.minReputation() {
		return nil, ErrProposalNotEligible
	}

	ip, err := s.ipRepo.FindByID(ipID)
	if err != nil || ip == nil {
		return nil, ErrProposalNotFound
	}

	if input.DescriptionChange != nil && *input.DescriptionChange == "" {
		input.DescriptionChange = nil
	}
	if input.CoverURLChange != nil && *input.CoverURLChange == "" {
		input.CoverURLChange = nil
	}
	hasChange := input.DescriptionChange != nil || input.CoverURLChange != nil ||
		len(input.TagsAdd) > 0 || len(input.TagsRemove) > 0
	if !hasChange {
		return nil, ErrProposalEmpty
	}

	currentTags, err := s.ipTags(ipID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, tag := range input.TagsAdd {
		if currentTags[tag] || seen[tag] {
			return nil, ErrProposalTagConflict
		}
		seen[tag] = true
	}
	for _, tag := range input.TagsRemove {
		if !currentTags[tag] || seen[tag] {
			return nil, ErrProposalTagConflict
		}
		seen[tag] = true
	}

	open, err := s.proposalRepo.FindOpenByIP(ipID)
	if err != nil {
		return nil, err
	}
	if open != nil {
		return nil, ErrProposalOpenExists
	}

	tagsAdd, _ := json.Marshal(input.TagsAdd)
	if input.TagsAdd == nil {
		tagsAdd = []byte("[]")
	}
	tagsRemove, _ := json.Marshal(input.TagsRemove)
	if input.TagsRemove == nil {
		tagsRemove = []byte("[]")
	}

	_, _, deadlineDays := s.thresholds()
	proposal := &model.IPProposal{
		IPID:              ipID,
		ProposerID:        proposerID,
		Status:            "open",
		DescriptionChange: input.DescriptionChange,
		CoverURLChange:    input.CoverURLChange,
		TagsAdd:           string(tagsAdd),
		TagsRemove:        string(tagsRemove),
		DeadlineAt:        time.Now().AddDate(0, 0, deadlineDays),
	}
	if err := s.proposalRepo.DB().Create(proposal).Error; err != nil {
		// 并发撞 uq_ip_proposals_open_per_ip 部分唯一索引 → 语义化 409。
		// gorm 仅在 TranslateError 时翻译 ErrDuplicatedKey（本项目未开启），
		// 故按既有先例（collection_repo.isCollectionItemDuplicateError）做字符串回退。
		if errors.Is(err, gorm.ErrDuplicatedKey) || isUniqueViolationError(err) {
			return nil, ErrProposalOpenExists
		}
		return nil, err
	}

	slog.Info("ip_proposal: created", "proposal_id", proposal.ID, "ip_id", ipID, "proposer_id", proposerID)

	// 提案文本走与 IP 创建相同的异步 fail-open AI 审核（story 23）
	s.submitProposalForAIReview(ctx, ip, proposal, proposerID, input)

	// 新提案 → 全体 IP 关注者（深链 target=ip，前端按 type 跳 ?tab=proposals）。
	// 异步执行：不在创建请求的关键路径上同步遍历关注者。
	recovery.GoSafe(func() {
		s.notifyProposalCreated(ip, proposal, proposerID)
	})
	return proposal, nil
}

// submitProposalForAIReview assembles the proposal's changed text and cover
// into one review submission. Fail-open: errors are logged, never block the
// proposal (mirrors ip_service.submitIPForAIReview).
func (s *IPProposalService) submitProposalForAIReview(ctx context.Context, ip *model.IP, proposal *model.IPProposal, proposerID int64, input CreateIPProposalInput) {
	if s.reviewSvc == nil {
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("IP 治理提案（%s）：", ip.Name))
	if input.DescriptionChange != nil {
		sb.WriteString("简介改为：")
		sb.WriteString(*input.DescriptionChange)
		sb.WriteString("；")
	}
	if input.CoverURLChange != nil {
		sb.WriteString("封面改为：")
		sb.WriteString(*input.CoverURLChange)
		sb.WriteString("；")
	}
	if len(input.TagsAdd) > 0 {
		sb.WriteString("新增标签：" + strings.Join(input.TagsAdd, ",") + "；")
	}
	if len(input.TagsRemove) > 0 {
		sb.WriteString("移除标签：" + strings.Join(input.TagsRemove, ",") + "；")
	}
	cover := ""
	if proposal.CoverURLChange != nil {
		cover = *proposal.CoverURLChange
	}
	reviewInput := SubmitReviewInput{
		TargetType:    "ip_proposal",
		TargetID:      proposal.ID,
		Title:         fmt.Sprintf("IP「%s」共治提案 #%d", ip.Name, proposal.ID),
		Description:   sb.String(),
		AuthorID:      proposerID,
		CoverImageURL: cover,
	}
	detachedCtx := context.WithoutCancel(ctx)
	recovery.GoSafe(func() {
		err := s.reviewSvc.SubmitForAIReview(detachedCtx, reviewInput)
		if err != nil && !errors.Is(err, aliyun.ErrGreenNotConfigured) {
			slog.Error("ip_proposal ai review failed", "proposal_id", proposal.ID, "error", err)
		}
	})
}

func (s *IPProposalService) notifyProposalCreated(ip *model.IP, proposal *model.IPProposal, proposerID int64) {
	if s.notify == nil {
		return
	}
	proposer := "someone"
	if u, err := s.userRepo.FindByID(proposerID); err == nil && u != nil {
		proposer = u.Username
	}
	title := fmt.Sprintf("IP「%s」有新的共治提案", ip.Name)
	body := fmt.Sprintf("%s 发起了资料修改提案，关注后即可参与投票。", proposer)
	const pageSize = 200
	for page := 1; ; page++ {
		followers, total, err := s.followRepo.GetFollowers("ip", ip.ID, page, pageSize)
		if err != nil {
			slog.Error("ip_proposal: list followers failed", "ip_id", ip.ID, "error", err)
			return
		}
		for _, f := range followers {
			s.fanout(f.ID, "ip_proposal_created", title, body, ip.ID, proposerID)
		}
		if int64(page*pageSize) >= total || len(followers) == 0 {
			return
		}
	}
}

// notifyClosed informs the proposer and every voter about the outcome.
func (s *IPProposalService) notifyClosed(ip *model.IP, proposal *model.IPProposal, status string) {
	if s.notify == nil {
		return
	}
	outcome := "已否决"
	notifType := "ip_proposal_rejected"
	if status == "adopted" {
		outcome = "已通过并生效"
		notifType = "ip_proposal_adopted"
	}
	title := fmt.Sprintf("IP「%s」的共治提案%s", ip.Name, outcome)
	body := fmt.Sprintf("表决结果：%d 赞成 / %d 反对。", proposal.YesVotes, proposal.NoVotes)
	seen := map[int64]bool{proposal.ProposerID: true}
	s.fanout(proposal.ProposerID, notifType, title, body, ip.ID, 0)
	voters, err := s.proposalRepo.ListVoterIDs(proposal.ID)
	if err != nil {
		slog.Error("ip_proposal: list voters failed", "proposal_id", proposal.ID, "error", err)
		return
	}
	for _, voterID := range voters {
		if seen[voterID] {
			continue
		}
		seen[voterID] = true
		s.fanout(voterID, notifType, title, body, ip.ID, 0)
	}
}

// closeExpired flips open proposals past their deadline to rejected and
// notifies their participants. Called lazily on every read so no cron is
// needed (MVP, #290).
func (s *IPProposalService) closeExpired(now time.Time) {
	var expired []model.IPProposal
	if err := s.proposalRepo.DB().
		Where("status = ? AND deadline_at < ?", "open", now).
		Limit(100).Find(&expired).Error; err != nil {
		return
	}
	for i := range expired {
		p := &expired[i]
		res := s.proposalRepo.DB().Model(&model.IPProposal{}).
			Where("id = ? AND status = ?", p.ID, "open").
			Updates(map[string]interface{}{"status": "rejected", "closed_at": now})
		if res.Error != nil || res.RowsAffected == 0 {
			continue
		}
		if ip, err := s.ipRepo.FindByID(p.IPID); err == nil && ip != nil {
			s.notifyClosed(ip, p, "rejected")
		}
	}
}

func (s *IPProposalService) myVote(proposalID, viewerID int64) *string {
	if viewerID <= 0 {
		return nil
	}
	vote, err := s.proposalRepo.FindVote(proposalID, viewerID)
	if err != nil || vote == nil {
		return nil
	}
	v := vote.Vote
	return &v
}

func (s *IPProposalService) GetProposal(ctx context.Context, proposalID, viewerID int64) (*IPProposalView, error) {
	s.closeExpired(time.Now())
	p, err := s.proposalRepo.FindByID(proposalID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrProposalNotFound
	}
	view := &IPProposalView{IPProposal: *p, MyVote: s.myVote(proposalID, viewerID)}
	if view.Proposer == nil {
		if u, err := s.userRepo.FindByID(p.ProposerID); err == nil && u != nil {
			view.Proposer = u
		}
	}
	return view, nil
}

// ListProposals returns one hub page plus the total under the same filter
// (tab count / search shrink, #290). query filters on the description
// change; status "all" spans statuses for counting; pageSize<=0 = 全量.
func (s *IPProposalService) ListProposals(ctx context.Context, ipID int64, status, query string, page, pageSize int, viewerID int64) ([]IPProposalView, int64, error) {
	s.closeExpired(time.Now())
	filter := repository.ListIPProposalsFilter{IPID: ipID, Status: status, Query: query, Page: page, PageSize: pageSize}
	rows, err := s.proposalRepo.List(filter)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.proposalRepo.Count(filter)
	if err != nil {
		return nil, 0, err
	}
	views := make([]IPProposalView, 0, len(rows))
	myVotes := map[int64]string{}
	if len(rows) > 0 {
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		if votes, err := s.proposalRepo.ListMyVotes(ids, viewerID); err == nil {
			for _, v := range votes {
				myVotes[v.ProposalID] = v.Vote
			}
		}
	}
	for i := range rows {
		view := IPProposalView{IPProposal: rows[i]}
		if v, ok := myVotes[rows[i].ID]; ok {
			vote := v
			view.MyVote = &vote
		}
		views = append(views, view)
	}
	return views, total, nil
}

// GovernanceDisplay exposes the configured voting bar (min votes + pass
// threshold) so the proposal cards render the scale from config instead of
// hardcoded constants. Nil-safe and honours effective-value fallbacks.
func (s *IPProposalService) GovernanceDisplay() (minVotes int, passThreshold float64) {
	minVotes, passThreshold, _ = s.thresholds()
	return minVotes, passThreshold
}

func (s *IPProposalService) ListVersions(ctx context.Context, ipID int64) ([]model.IPProfileVersion, error) {
	return s.proposalRepo.ListVersions(ipID)
}

// SubmitVote casts an immutable yes/no vote. When the vote crosses
// min_votes + pass_threshold the proposal adopts inside the same transaction:
// the IP profile is updated, a version snapshot with the OLD values is
// written, the IP cache is invalidated, and the proposal closes as adopted.
func (s *IPProposalService) SubmitVote(ctx context.Context, proposalID, voterID int64, vote string) error {
	if vote != "yes" && vote != "no" {
		return errors.New("invalid vote")
	}
	user, err := s.userRepo.FindByID(voterID)
	if err != nil || user == nil {
		return ErrProposalNotEligible
	}
	if user.IsBanned || user.Reputation < s.minReputation() {
		return ErrProposalNotEligible
	}

	db := s.proposalRepo.DB()
	adopted := false
	txErr := db.Transaction(func(tx *gorm.DB) error {
		var proposal model.IPProposal
		if err := tx.First(&proposal, proposalID).Error; err != nil {
			return ErrProposalNotFound
		}
		if proposal.Status != "open" || proposal.DeadlineAt.Before(time.Now()) {
			// 过期惰性结案由读路径 closeExpired 兜底（事务内改写会随
			// ErrProposalClosed 一起回滚，成为死写），此处只读判定。
			return ErrProposalClosed
		}

		var existing model.IPProposalVote
		err := tx.Where("proposal_id = ? AND voter_id = ?", proposalID, voterID).First(&existing).Error
		if err == nil {
			return ErrProposalAlreadyVoted
		}
		// 用 tx 直查（而非主连接 repo），避免单连接测试环境下的自锁
		var followCount int64
		if err := tx.Model(&model.Follow{}).
			Where("follower_id = ? AND target_type = ? AND target_id = ?", voterID, "ip", proposal.IPID).
			Count(&followCount).Error; err != nil {
			return err
		}
		if followCount == 0 {
			return ErrProposalNotEligible
		}

		if err := tx.Create(&model.IPProposalVote{ProposalID: proposalID, VoterID: voterID, Vote: vote}).Error; err != nil {
			return err
		}
		// 原子自增防并发丢更新；随后重读拿最新计数做门槛判定
		counts := map[string]interface{}{}
		if vote == "yes" {
			counts["yes_votes"] = gorm.Expr("yes_votes + 1")
		} else {
			counts["no_votes"] = gorm.Expr("no_votes + 1")
		}
		if err := tx.Model(&model.IPProposal{}).Where("id = ?", proposalID).Updates(counts).Error; err != nil {
			return err
		}
		if err := tx.First(&proposal, proposalID).Error; err != nil {
			return err
		}

		minVotes, threshold, _ := s.thresholds()
		total := proposal.YesVotes + proposal.NoVotes
		if proposal.YesVotes >= minVotes && float64(proposal.YesVotes)/float64(total) >= threshold {
			return s.adoptTx(tx, &proposal, &adopted)
		}
		return nil
	})
	if txErr == nil && adopted {
		// 缓存失效与结案通知都在事务提交后执行：失效提前会让并发读在提交
		// 前把旧资料回填进缓存。
		if fresh, err := s.proposalRepo.FindByID(proposalID); err == nil && fresh != nil {
			s.invalidateIPCache(fresh.IPID)
			if ip, ipErr := s.ipRepo.FindByID(fresh.IPID); ipErr == nil && ip != nil {
				s.notifyClosed(ip, fresh, "adopted")
			}
		}
	}
	return txErr
}

// adoptTx applies the proposal to the IP profile, snapshots the old values,
// and closes the proposal — all inside the caller's transaction. The status
// flip is a guarded claim (WHERE status='open') so concurrent deciding votes
// produce exactly one adoption; the loser keeps its counted vote and skips
// the side effects.
func (s *IPProposalService) adoptTx(tx *gorm.DB, proposal *model.IPProposal, adopted *bool) error {
	now := time.Now()
	claim := tx.Model(&model.IPProposal{}).Where("id = ? AND status = ?", proposal.ID, "open").
		Updates(map[string]interface{}{
			"status": "adopted", "closed_at": now, "effective_at": now,
			"yes_votes": proposal.YesVotes, "no_votes": proposal.NoVotes,
		})
	if claim.Error != nil {
		return claim.Error
	}
	if claim.RowsAffected == 0 {
		return nil
	}
	*adopted = true

	var ip model.IP
	if err := tx.First(&ip, proposal.IPID).Error; err != nil {
		return err
	}

	var tagsAdd, tagsRemove []string
	_ = json.Unmarshal([]byte(proposal.TagsAdd), &tagsAdd)
	_ = json.Unmarshal([]byte(proposal.TagsRemove), &tagsRemove)

	oldTags := []string{}
	var tagRows []model.IPTag
	tx.Where("ip_id = ?", ip.ID).Find(&tagRows)
	for _, row := range tagRows {
		oldTags = append(oldTags, row.Tag)
	}

	changes := map[string]interface{}{}
	updates := map[string]interface{}{}
	if proposal.DescriptionChange != nil {
		changes["description"] = *proposal.DescriptionChange
		updates["description"] = *proposal.DescriptionChange
	}
	if proposal.CoverURLChange != nil {
		changes["cover_url"] = *proposal.CoverURLChange
		updates["cover_url"] = *proposal.CoverURLChange
	}
	if len(tagsAdd) > 0 || len(tagsRemove) > 0 {
		changes["tags_add"] = tagsAdd
		changes["tags_remove"] = tagsRemove
	}

	if len(updates) > 0 {
		if err := tx.Model(&model.IP{}).Where("id = ?", ip.ID).Updates(updates).Error; err != nil {
			return err
		}
	}
	if len(tagsAdd) > 0 {
		rows := make([]model.IPTag, 0, len(tagsAdd))
		for _, tag := range tagsAdd {
			rows = append(rows, model.IPTag{IPID: ip.ID, Tag: tag})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(tagsRemove) > 0 {
		if err := tx.Where("ip_id = ? AND tag IN ?", ip.ID, tagsRemove).Delete(&model.IPTag{}).Error; err != nil {
			return err
		}
	}

	snapshot := map[string]interface{}{
		"description": ip.Description,
		"cover_url":   ip.CoverURL,
		"tags":        oldTags,
	}
	snapJSON, _ := json.Marshal(snapshot)
	changesJSON, _ := json.Marshal(changes)
	if err := tx.Create(&model.IPProfileVersion{
		IPID: ip.ID, ProposalID: proposal.ID,
		Snapshot: string(snapJSON), Changes: string(changesJSON),
		YesVotes: proposal.YesVotes, NoVotes: proposal.NoVotes, CreatedAt: now,
	}).Error; err != nil {
		return err
	}

	// 缓存失效由调用方在事务提交后执行（见 SubmitVote）。
	slog.Info("ip_proposal: adopted", "proposal_id", proposal.ID, "ip_id", ip.ID,
		"yes", proposal.YesVotes, "no", proposal.NoVotes)
	return nil
}

// isUniqueViolationError recognizes raw driver unique-violation errors for
// dialects where gorm does not translate them (TranslateError off).
func isUniqueViolationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique constraint failed") ||
		strings.Contains(msg, "violates unique constraint")
}

func (s *IPProposalService) invalidateIPCache(ipID int64) {
	if s.rdb == nil {
		return
	}
	if err := s.rdb.Del(context.Background(), fmt.Sprintf("cache:ip:%d", ipID)).Err(); err != nil {
		slog.Error("ip_proposal: cache invalidation failed", "ip_id", ipID, "error", err)
	}
}
