package repository

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strconv"
	"sync"
	"time"

	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrDMReplyRequired = errors.New("dm reply required")

var conversationPairLocks = struct {
	sync.Mutex
	locks map[string]*conversationPairLock
}{
	locks: map[string]*conversationPairLock{},
}

type conversationPairLock struct {
	sync.Mutex
	refs int
}

type MessageRepository struct {
	db *gorm.DB
}

type ConversationParticipantSummary struct {
	ID        int64
	Username  string
	AvatarURL string
}

type ConversationSummary struct {
	ID           int64
	Participants []ConversationParticipantSummary
	LastMessage  *model.Message
	UnreadCount  int
	UpdatedAt    time.Time
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) FindOrCreateConversation(userA, userB int64) (int64, error) {
	var convID int64
	err := r.withConversationPairTransaction(userA, userB, func(txRepo *MessageRepository) error {
		var err error
		convID, err = txRepo.findOrCreateConversation(userA, userB)
		return err
	})
	return convID, err
}

func (r *MessageRepository) findOrCreateConversation(userA, userB int64) (int64, error) {
	var convID int64
	err := r.db.Raw(`
		SELECT c.id FROM conversations c
		JOIN conversation_participants p1 ON p1.conversation_id = c.id AND p1.user_id = ? AND p1.left_at IS NULL
		JOIN conversation_participants p2 ON p2.conversation_id = c.id AND p2.user_id = ? AND p2.left_at IS NULL
		LIMIT 1
	`, userA, userB).Scan(&convID).Error
	if err != nil {
		return 0, err
	}
	if convID > 0 {
		return convID, nil
	}

	conv := model.Conversation{}
	if err := r.db.Create(&conv).Error; err != nil {
		return 0, err
	}
	convID = conv.ID
	p1 := model.ConversationParticipant{ConversationID: conv.ID, UserID: userA}
	p2 := model.ConversationParticipant{ConversationID: conv.ID, UserID: userB}
	return convID, r.db.Create(&p1).Create(&p2).Error
}

func (r *MessageRepository) ListConversations(userID int64, page, pageSize int) ([]model.Conversation, error) {
	var convIDs []int64
	r.db.Model(&model.ConversationParticipant{}).
		Select("conversation_id").Where("user_id = ? AND left_at IS NULL", userID).
		Pluck("conversation_id", &convIDs)

	var conversations []model.Conversation
	if len(convIDs) == 0 {
		return conversations, nil
	}
	err := r.db.Where("id IN ?", convIDs).
		Order("updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&conversations).Error
	return conversations, err
}

func (r *MessageRepository) ListConversationSummaries(userID int64, page, pageSize int) ([]ConversationSummary, error) {
	var rows []struct {
		ID          int64
		UpdatedAt   time.Time
		UnreadCount int
	}
	err := r.db.Table("conversations AS c").
		Select("c.id, c.updated_at, cp.unread_count").
		Joins("JOIN conversation_participants AS cp ON cp.conversation_id = c.id AND cp.user_id = ? AND cp.left_at IS NULL", userID).
		Order("c.updated_at DESC, c.id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return []ConversationSummary{}, err
	}

	summaries := make([]ConversationSummary, len(rows))
	summaryByID := make(map[int64]*ConversationSummary, len(rows))
	convIDs := make([]int64, 0, len(rows))
	for i, row := range rows {
		summaries[i] = ConversationSummary{
			ID:          row.ID,
			UnreadCount: row.UnreadCount,
			UpdatedAt:   row.UpdatedAt,
		}
		summaryByID[row.ID] = &summaries[i]
		convIDs = append(convIDs, row.ID)
	}

	var participantRows []struct {
		ConversationID int64
		ID             int64
		Username       string
		AvatarURL      string
	}
	if err := r.db.Table("conversation_participants AS cp").
		Select("cp.conversation_id, u.id, u.username, u.avatar_url").
		Joins("JOIN users AS u ON u.id = cp.user_id").
		Where("cp.conversation_id IN ? AND cp.left_at IS NULL AND cp.user_id <> ?", convIDs, userID).
		Order("cp.conversation_id ASC, u.id ASC").
		Scan(&participantRows).Error; err != nil {
		return nil, err
	}
	for _, row := range participantRows {
		if summary := summaryByID[row.ConversationID]; summary != nil {
			summary.Participants = append(summary.Participants, ConversationParticipantSummary{
				ID:        row.ID,
				Username:  row.Username,
				AvatarURL: row.AvatarURL,
			})
		}
	}

	var messages []model.Message
	if err := r.db.Raw(`
		SELECT id, conversation_id, sender_id, body, created_at
		FROM (
			SELECT
				m.id,
				m.conversation_id,
				m.sender_id,
				m.body,
				m.created_at,
				ROW_NUMBER() OVER (PARTITION BY m.conversation_id ORDER BY m.created_at DESC, m.id DESC) AS rn
			FROM messages AS m
			WHERE m.conversation_id IN ?
		) AS ranked
		WHERE rn = 1
	`, convIDs).Scan(&messages).Error; err != nil {
		return nil, err
	}
	for i := range messages {
		if summary := summaryByID[messages[i].ConversationID]; summary != nil {
			message := messages[i]
			summary.LastMessage = &message
		}
	}

	return summaries, nil
}

func (r *MessageRepository) ListMessages(convID int64, page, pageSize int) ([]model.Message, int64, error) {
	var total int64
	r.db.Model(&model.Message{}).Where("conversation_id = ?", convID).Count(&total)
	var messages []model.Message
	err := r.db.Where("conversation_id = ?", convID).
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&messages).Error
	return messages, total, err
}

func (r *MessageRepository) CountMessagesFromOtherParticipant(convID, currentUserID int64) (int64, error) {
	otherUserID, ok, err := r.otherParticipantID(convID, currentUserID)
	if err != nil || !ok {
		return 0, err
	}
	var count int64
	err = r.db.Model(&model.Message{}).
		Where("conversation_id IN (?) AND sender_id != ?", r.conversationIDsForPairQuery(currentUserID, otherUserID), currentUserID).
		Count(&count).Error
	return count, err
}

func (r *MessageRepository) LastMessageSender(convID int64) (*int64, error) {
	participantIDs, err := r.participantIDs(convID)
	if err != nil {
		return nil, err
	}
	messageScope := r.db.Model(&model.Message{}).Where("conversation_id = ?", convID)
	if len(participantIDs) >= 2 {
		messageScope = r.db.Model(&model.Message{}).
			Where("conversation_id IN (?)", r.conversationIDsForPairQuery(participantIDs[0], participantIDs[1]))
	}

	var rows []struct {
		SenderID int64
	}
	err = messageScope.
		Select("sender_id").
		Order("created_at DESC, id DESC").
		Limit(1).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	senderID := rows[0].SenderID
	return &senderID, nil
}

func (r *MessageRepository) SendWithColdStartGuard(senderID, recipientID int64, body string) (*model.Message, error) {
	var msg *model.Message
	err := r.withConversationPairTransaction(senderID, recipientID, func(txRepo *MessageRepository) error {
		convID, err := txRepo.findOrCreateConversation(senderID, recipientID)
		if err != nil {
			return err
		}
		if err := txRepo.lockConversation(convID); err != nil {
			return err
		}

		otherMessageCount, err := txRepo.CountMessagesFromOtherParticipant(convID, senderID)
		if err != nil {
			return err
		}
		if otherMessageCount == 0 {
			lastSenderID, err := txRepo.LastMessageSender(convID)
			if err != nil {
				return err
			}
			if lastSenderID != nil && *lastSenderID == senderID {
				return ErrDMReplyRequired
			}
		}

		msg, err = txRepo.sendInTx(senderID, convID, body)
		return err
	})
	return msg, err
}

func (r *MessageRepository) Send(senderID, convID int64, body string) (*model.Message, error) {
	var msg *model.Message
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var err error
		msg, err = (&MessageRepository{db: tx}).sendInTx(senderID, convID, body)
		return err
	})
	return msg, err
}

func (r *MessageRepository) sendInTx(senderID, convID int64, body string) (*model.Message, error) {
	msg := &model.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Body:           body,
	}
	if err := r.db.Create(msg).Error; err != nil {
		return nil, err
	}
	if err := r.db.Model(&model.Conversation{}).Where("id = ?", convID).Update("updated_at", msg.CreatedAt).Error; err != nil {
		return nil, err
	}
	return msg, r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id != ?", convID, senderID).
		UpdateColumn("unread_count", gorm.Expr("unread_count + 1")).Error
}

func (r *MessageRepository) IsParticipant(userID, convID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.ConversationParticipant{}).
		Where("user_id = ? AND conversation_id = ? AND left_at IS NULL", userID, convID).
		Count(&count).Error
	return count > 0, err
}

func (r *MessageRepository) UpdateLastRead(userID, convID int64) error {
	return r.db.Model(&model.ConversationParticipant{}).
		Where("user_id = ? AND conversation_id = ?", userID, convID).
		Updates(map[string]interface{}{
			"last_read_at": time.Now().UTC(),
			"unread_count": 0,
		}).Error
}

func (r *MessageRepository) DeleteMessage(msgID, userID int64) error {
	return r.db.Model(&model.Message{}).Where("id = ? AND sender_id = ?", msgID, userID).
		Update("body", "[message deleted]").Error
}

func (r *MessageRepository) LeaveConversation(convID, userID int64) error {
	return r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id = ?", convID, userID).
		Update("left_at", gorm.Expr("NOW()")).Error
}

func (r *MessageRepository) otherParticipantID(convID, currentUserID int64) (int64, bool, error) {
	var ids []int64
	err := r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ? AND user_id != ?", convID, currentUserID).
		Limit(1).
		Pluck("user_id", &ids).Error
	if err != nil || len(ids) == 0 {
		return 0, false, err
	}
	return ids[0], true, nil
}

func (r *MessageRepository) participantIDs(convID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&model.ConversationParticipant{}).
		Where("conversation_id = ?", convID).
		Order("user_id ASC").
		Limit(2).
		Pluck("user_id", &ids).Error
	return ids, err
}

func (r *MessageRepository) conversationIDsForPairQuery(userA, userB int64) *gorm.DB {
	return r.db.Table("conversation_participants AS p1").
		Select("p1.conversation_id").
		Joins("JOIN conversation_participants AS p2 ON p2.conversation_id = p1.conversation_id AND p2.user_id = ?", userB).
		Where("p1.user_id = ?", userA)
}

func (r *MessageRepository) withConversationPairTransaction(userA, userB int64, fn func(txRepo *MessageRepository) error) error {
	if usesPostgresConversationPairLock(r.db) {
		return r.db.Transaction(func(tx *gorm.DB) error {
			txRepo := &MessageRepository{db: tx}
			if err := txRepo.lockPostgresConversationPair(userA, userB); err != nil {
				return err
			}
			return fn(txRepo)
		})
	}

	unlockPair := lockConversationPairInProcess(conversationPairKey(userA, userB))
	defer unlockPair()
	return r.db.Transaction(func(tx *gorm.DB) error {
		return fn(&MessageRepository{db: tx})
	})
}

func (r *MessageRepository) lockPostgresConversationPair(userA, userB int64) error {
	return r.db.Exec("SELECT pg_advisory_xact_lock(?)", conversationPairAdvisoryKey(conversationPairKey(userA, userB))).Error
}

func (r *MessageRepository) lockConversation(convID int64) error {
	if !supportsConversationRowLock(r.db) {
		return nil
	}
	var conversation model.Conversation
	return r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("id = ?", convID).
		Take(&conversation).Error
}

func supportsConversationRowLock(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() != "sqlite"
}

func usesPostgresConversationPairLock(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Dialector.Name() == "postgres"
}

func conversationPairKey(userA, userB int64) string {
	if userA > userB {
		userA, userB = userB, userA
	}
	return strconv.FormatInt(userA, 10) + ":" + strconv.FormatInt(userB, 10)
}

func conversationPairAdvisoryKey(pairKey string) int64 {
	// Transaction advisory locks need one stable int64; a namespaced hash keeps pair IDs canonical without extra schema.
	sum := sha256.Sum256([]byte("dm-conversation:" + pairKey))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func lockConversationPairInProcess(pairKey string) func() {
	conversationPairLocks.Lock()
	lock := conversationPairLocks.locks[pairKey]
	if lock == nil {
		lock = &conversationPairLock{}
		conversationPairLocks.locks[pairKey] = lock
	}
	lock.refs++
	conversationPairLocks.Unlock()

	lock.Lock()
	return func() {
		lock.Unlock()

		conversationPairLocks.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(conversationPairLocks.locks, pairKey)
		}
		conversationPairLocks.Unlock()
	}
}
