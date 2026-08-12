package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrInviteSelfNotAllowed       = errors.New("INVITE_SELF_NOT_ALLOWED")
	ErrInviteAuthorNotAllowed     = errors.New("INVITE_AUTHOR_NOT_ALLOWED")
	ErrInviteAlreadyContributor   = errors.New("INVITE_ALREADY_CONTRIBUTOR")
	ErrInviteeUnavailable         = errors.New("INVITEE_UNAVAILABLE")
	ErrContentUnavailable         = errors.New("CONTENT_UNAVAILABLE")
	ErrContributorLimitReached    = errors.New("CONTRIBUTOR_LIMIT_REACHED")
	ErrNotContentOwner            = errors.New("NOT_CONTENT_OWNER")
	ErrReputationTooLow           = errors.New("REPUTATION_TOO_LOW")
	ErrInviteDailyLimit           = errors.New("INVITE_DAILY_LIMIT")
	ErrInviteDuplicateUser        = errors.New("INVITE_DUPLICATE_USER")
	ErrInviteBlocked              = errors.New("INVITE_BLOCKED")
	ErrInviteNotAccepting         = errors.New("INVITE_NOT_ACCEPTING")
	ErrInviteAlreadyExists        = errors.New("INVITE_ALREADY_EXISTS")
	ErrInviteRateLimitUnavailable = errors.New("INVITE_SERVICE_UNAVAILABLE")
	ErrInviteNotFound             = errors.New("INVITE_NOT_FOUND")
	ErrInviteExpired              = errors.New("INVITE_EXPIRED")
	ErrInviteNotInvitee           = errors.New("INVITE_NOT_INVITEE")
	ErrInviteNotPending           = errors.New("INVITE_NOT_PENDING")
)

const (
	collabInviteKeyTTLSeconds  = 86400
	collabInviteMessageBody    = "联合创作邀请"
	collabInviteMessageType    = "collab_invite"
	inviteReserveLimitExceeded = 1
	inviteReserveDuplicateUser = 2
)

// inviteReserveScript atomically reserves one invite against the inviter's
// daily counter and the per-invitee key for the Asia/Shanghai calendar day.
// KEYS[1] = daily count key, KEYS[2] = per-invitee key; ARGV[1] = daily limit,
// ARGV[2] = TTL seconds, ARGV[3] = reservation token. Returns 0 when reserved,
// 1 when the daily limit is reached, 2 when the invitee was already invited
// today. A rejection never modifies either key, so a failed reservation cannot
// leave a partial quota behind.
var inviteReserveScript = redis.NewScript(`
local count = redis.call("GET", KEYS[1])
if count and tonumber(count) >= tonumber(ARGV[1]) then
  return 1
end
if redis.call("EXISTS", KEYS[2]) == 1 then
  return 2
end
local next = redis.call("INCR", KEYS[1])
if next == 1 then redis.call("EXPIRE", KEYS[1], ARGV[2]) end
redis.call("SET", KEYS[2], ARGV[3])
redis.call("EXPIRE", KEYS[2], ARGV[2])
return 0
`)

// inviteCompensateScript releases a reservation after a DB failure. It never
// decrements below zero (deleting the counter at zero) and deletes the
// per-invitee key only when it still holds the reservation token, so a
// concurrent reservation for the same invitee is never released.
// KEYS[1] = daily count key, KEYS[2] = per-invitee key; ARGV[1] = token.
var inviteCompensateScript = redis.NewScript(`
local count = redis.call("GET", KEYS[1])
if count then
  local next = tonumber(count) - 1
  if next <= 0 then
    redis.call("DEL", KEYS[1])
  else
    redis.call("SET", KEYS[1], next)
  end
end
if redis.call("GET", KEYS[2]) == ARGV[1] then
  redis.call("DEL", KEYS[2])
end
return 0
`)

// CollabInviteService runs the collaboration invite anti-abuse chain and the
// invite creation transaction.
type CollabInviteService struct {
	contentRepo *repository.ContentRepository
	inviteRepo  *repository.CollabInviteRepository
	messageRepo *repository.MessageRepository
	userRepo    *repository.UserRepository
	rdb         *redis.Client
	cfg         *config.Config
	now         func() time.Time
}

func NewCollabInviteService(
	contentRepo *repository.ContentRepository,
	inviteRepo *repository.CollabInviteRepository,
	messageRepo *repository.MessageRepository,
	userRepo *repository.UserRepository,
	rdb *redis.Client,
	cfg *config.Config,
) *CollabInviteService {
	return &CollabInviteService{
		contentRepo: contentRepo,
		inviteRepo:  inviteRepo,
		messageRepo: messageRepo,
		userRepo:    userRepo,
		rdb:         rdb,
		cfg:         cfg,
		now:         time.Now,
	}
}

// SendInvite validates the invite against the full anti-abuse chain, reserves
// the Redis daily quota fail-closed, then creates the invite, the 1:1
// conversation and the typed invite card message in one transaction. When the
// DB transaction fails after a successful reservation, the reservation is
// compensated best-effort before the DB error is returned.
func (s *CollabInviteService) SendInvite(ctx context.Context, contentID, inviterID, inviteeID int64) (*model.CollabInvite, error) {
	if s.rdb == nil {
		return nil, ErrInviteRateLimitUnavailable
	}
	if inviterID == inviteeID {
		return nil, ErrInviteSelfNotAllowed
	}

	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return nil, err
	}
	if content == nil || !invitableContentStatus(content.Status) {
		return nil, ErrContentUnavailable
	}
	if content.AuthorID != inviterID {
		contributor, err := s.contentRepo.IsContributor(contentID, inviterID)
		if err != nil {
			return nil, err
		}
		if !contributor {
			return nil, ErrNotContentOwner
		}
	}

	invitee, err := s.userRepo.FindByID(inviteeID)
	if err != nil {
		return nil, err
	}
	if invitee == nil || invitee.IsBanned || invitee.DeletedAt != nil {
		return nil, ErrInviteeUnavailable
	}
	if inviteeID == content.AuthorID {
		return nil, ErrInviteAuthorNotAllowed
	}
	alreadyContributor, err := s.contentRepo.IsContributor(contentID, inviteeID)
	if err != nil {
		return nil, err
	}
	if alreadyContributor {
		return nil, ErrInviteAlreadyContributor
	}

	contributors, err := s.contentRepo.CountContributors(contentID)
	if err != nil {
		return nil, err
	}
	pending, err := s.contentRepo.CountPendingInviteesNotContributors(contentID)
	if err != nil {
		return nil, err
	}
	if contributors+pending >= int64(s.maxContributorsPerItem()) {
		return nil, ErrContributorLimitReached
	}

	inviter, err := s.userRepo.FindByID(inviterID)
	if err != nil {
		return nil, err
	}
	if inviter == nil || inviter.IsBanned || inviter.DeletedAt != nil {
		return nil, ErrNotContentOwner
	}
	if inviter.Reputation < minScoreForInteraction(s.cfg) {
		return nil, ErrReputationTooLow
	}

	blocked, err := s.inviteRepo.IsBlockedByEither(inviterID, inviteeID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrInviteBlocked
	}
	if !invitee.AcceptCollabInvites {
		return nil, ErrInviteNotAccepting
	}
	active, err := s.inviteRepo.HasActiveInvite(contentID, inviteeID)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, ErrInviteAlreadyExists
	}

	day := s.now().In(inviteShanghaiLocation())
	countKey := inviteDailyCountKey(inviterID, day)
	userKey := invitePerUserKey(inviterID, inviteeID, day)
	limit := s.inviteDailyLimit()
	token := inviteReservationToken()

	reserved, err := inviteReserveScript.Run(ctx, s.rdb, []string{countKey, userKey}, limit, collabInviteKeyTTLSeconds, token).Int()
	if err != nil {
		slog.Warn("[collab_invite] Redis quota reservation unavailable, failing closed", "error", err)
		return nil, ErrInviteRateLimitUnavailable
	}
	switch reserved {
	case inviteReserveLimitExceeded:
		return nil, ErrInviteDailyLimit
	case inviteReserveDuplicateUser:
		return nil, ErrInviteDuplicateUser
	}

	invite := &model.CollabInvite{
		ContentID: contentID,
		InviterID: inviterID,
		InviteeID: inviteeID,
		Status:    model.CollabInviteStatusPending,
		ExpiresAt: s.now().Add(time.Duration(s.inviteExpireDays()) * 24 * time.Hour),
	}
	metadata := model.JSONMap{
		"content_id":       contentID,
		"content_title":    content.Title,
		"inviter_id":       inviterID,
		"inviter_username": inviter.Username,
	}

	err = s.contentRepo.Transaction(func(txContent *repository.ContentRepository) error {
		tx := txContent.DB()

		locked, err := txContent.FindByIDForUpdate(contentID)
		if err != nil {
			return err
		}
		if locked == nil || !invitableContentStatus(locked.Status) {
			return ErrContentUnavailable
		}
		contributor, err := txContent.IsContributor(contentID, inviteeID)
		if err != nil {
			return err
		}
		if contributor {
			return ErrInviteAlreadyContributor
		}
		contributors, err := txContent.CountContributors(contentID)
		if err != nil {
			return err
		}
		pending, err := txContent.CountPendingInviteesNotContributors(contentID)
		if err != nil {
			return err
		}
		if contributors+pending >= int64(s.maxContributorsPerItem()) {
			return ErrContributorLimitReached
		}

		inviteRepo := repository.NewCollabInviteRepository(tx)
		if err := inviteRepo.CreateInvite(invite); err != nil {
			if isActiveCollabInviteUniqueViolation(err) {
				return ErrInviteAlreadyExists
			}
			return err
		}
		metadata["invite_id"] = invite.ID

		convID, err := s.messageRepo.FindOrCreateConversationTx(tx, inviterID, inviteeID)
		if err != nil {
			return err
		}
		msg, err := s.messageRepo.SendTypedTx(tx, inviterID, convID, collabInviteMessageBody, collabInviteMessageType, metadata)
		if err != nil {
			return err
		}
		invite.MessageID = &msg.ID
		return inviteRepo.UpdateMessageID(invite.ID, msg.ID)
	})
	if err != nil {
		s.compensateInviteReservation(ctx, countKey, userKey, token)
		return nil, err
	}
	return invite, nil
}

// AcceptInvite accepts a pending invite: it records the invitee as a
// contributor (idempotently, pr_count=0, never touching an existing row's
// pr_count or first_at) and transitions the invite to accepted. The invite
// row is locked first, then its content_items parent row, so concurrent
// accepts for the same content serialize on the content lock and can never
// exceed max_contributors_per_item. Capacity failures roll back the whole
// transaction, leaving the invite pending.
func (s *CollabInviteService) AcceptInvite(ctx context.Context, inviteID, userID int64) (*model.CollabInvite, error) {
	var accepted *model.CollabInvite
	err := s.contentRepo.Transaction(func(txContent *repository.ContentRepository) error {
		tx := txContent.DB()
		inviteRepo := repository.NewCollabInviteRepository(tx)

		invite, err := inviteRepo.FindByIDForUpdate(inviteID)
		if err != nil {
			return err
		}
		if invite == nil {
			return ErrInviteNotFound
		}
		if invite.InviteeID != userID {
			return ErrInviteNotInvitee
		}
		if invite.Status != model.CollabInviteStatusPending {
			return ErrInviteNotPending
		}
		now := s.now()
		if !invite.ExpiresAt.After(now) {
			return ErrInviteExpired
		}

		content, err := txContent.FindByIDForUpdate(invite.ContentID)
		if err != nil {
			return err
		}
		if content == nil {
			return ErrContentUnavailable
		}
		alreadyContributor, err := txContent.IsContributor(invite.ContentID, userID)
		if err != nil {
			return err
		}
		if !alreadyContributor {
			contributors, err := txContent.CountContributors(invite.ContentID)
			if err != nil {
				return err
			}
			if contributors >= int64(s.maxContributorsPerItem()) {
				return ErrContributorLimitReached
			}
		}
		if err := txContent.InsertContributorIfAbsent(invite.ContentID, userID, now); err != nil {
			return err
		}

		invite.Status = model.CollabInviteStatusAccepted
		invite.RespondedAt = &now
		if err := inviteRepo.UpdateStatus(invite.ID, model.CollabInviteStatusAccepted, &now); err != nil {
			return err
		}
		accepted = invite
		return nil
	})
	if err != nil {
		return nil, err
	}
	return accepted, nil
}

// DeclineInvite declines a pending invite: it locks the invite row, verifies
// the caller is the invitee and the invite is still pending and unexpired,
// then transitions it to declined with responded_at. No content row lock is
// needed because declining does not touch contributor capacity.
func (s *CollabInviteService) DeclineInvite(ctx context.Context, inviteID, userID int64) (*model.CollabInvite, error) {
	var declined *model.CollabInvite
	err := s.contentRepo.Transaction(func(txContent *repository.ContentRepository) error {
		tx := txContent.DB()
		inviteRepo := repository.NewCollabInviteRepository(tx)

		invite, err := inviteRepo.FindByIDForUpdate(inviteID)
		if err != nil {
			return err
		}
		if invite == nil {
			return ErrInviteNotFound
		}
		if invite.InviteeID != userID {
			return ErrInviteNotInvitee
		}
		if invite.Status != model.CollabInviteStatusPending {
			return ErrInviteNotPending
		}
		now := s.now()
		if !invite.ExpiresAt.After(now) {
			return ErrInviteExpired
		}

		invite.Status = model.CollabInviteStatusDeclined
		invite.RespondedAt = &now
		if err := inviteRepo.UpdateStatus(invite.ID, model.CollabInviteStatusDeclined, &now); err != nil {
			return err
		}
		declined = invite
		return nil
	})
	if err != nil {
		return nil, err
	}
	return declined, nil
}

func (s *CollabInviteService) compensateInviteReservation(ctx context.Context, countKey, userKey, token string) {
	if s.rdb == nil {
		return
	}
	if err := inviteCompensateScript.Run(ctx, s.rdb, []string{countKey, userKey}, token).Err(); err != nil {
		slog.Error("[collab_invite] failed to compensate Redis reservation after DB failure",
			"count_key", countKey, "user_key", userKey, "error", err)
	}
}

func (s *CollabInviteService) inviteDailyLimit() int {
	if s.cfg != nil && s.cfg.Collaboration.InviteDailyLimit > 0 {
		return s.cfg.Collaboration.InviteDailyLimit
	}
	return 20
}

func (s *CollabInviteService) inviteExpireDays() int {
	if s.cfg != nil && s.cfg.Collaboration.InviteExpireDays > 0 {
		return s.cfg.Collaboration.InviteExpireDays
	}
	return 7
}

func (s *CollabInviteService) maxContributorsPerItem() int {
	if s.cfg != nil && s.cfg.Collaboration.MaxContributorsPerItem > 0 {
		return s.cfg.Collaboration.MaxContributorsPerItem
	}
	return 10
}

func inviteDailyCountKey(inviterID int64, day time.Time) string {
	return fmt.Sprintf("collab_invite_count:%d:%s", inviterID, day.Format("2006-01-02"))
}

func invitePerUserKey(inviterID, inviteeID int64, day time.Time) string {
	return fmt.Sprintf("collab_invite_user:%d:%d:%s", inviterID, inviteeID, day.Format("2006-01-02"))
}

func inviteShanghaiLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func inviteReservationToken() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

func isActiveCollabInviteUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == postgresUniqueViolationSQLState && pgErr.ConstraintName == "idx_collab_invites_active"
	}
	return false
}

func invitableContentStatus(status string) bool {
	return status == "pending" || status == "published"
}
