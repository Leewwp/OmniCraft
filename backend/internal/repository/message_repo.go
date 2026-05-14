package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) FindOrCreateConversation(userA, userB int64) (int64, error) {
	var convID int64
	err := r.db.Raw(`
		SELECT c.id FROM conversations c
		JOIN conversation_participants p1 ON p1.conversation_id = c.id AND p1.user_id = ?
		JOIN conversation_participants p2 ON p2.conversation_id = c.id AND p2.user_id = ?
		LIMIT 1
	`, userA, userB).Scan(&convID).Error
	if err != nil {
		return 0, err
	}
	if convID > 0 {
		return convID, nil
	}

	return convID, r.db.Transaction(func(tx *gorm.DB) error {
		conv := model.Conversation{}
		if err := tx.Create(&conv).Error; err != nil {
			return err
		}
		convID = conv.ID
		p1 := model.ConversationParticipant{ConversationID: conv.ID, UserID: userA}
		p2 := model.ConversationParticipant{ConversationID: conv.ID, UserID: userB}
		return tx.Create(&p1).Create(&p2).Error
	})
}

func (r *MessageRepository) ListConversations(userID int64, page, pageSize int) ([]model.Conversation, error) {
	var convIDs []int64
	r.db.Model(&model.ConversationParticipant{}).
		Select("conversation_id").Where("user_id = ?", userID).
		Pluck("conversation_id", &convIDs)

	var conversations []model.Conversation
	if len(convIDs) == 0 {
		return conversations, nil
	}
	err := r.db.Where("id IN ?", convIDs).
		Order("updated_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&conversations).Error
	return conversations, err
}

func (r *MessageRepository) ListMessages(convID int64, page, pageSize int) ([]model.Message, int64, error) {
	var total int64
	r.db.Model(&model.Message{}).Where("conversation_id = ?", convID).Count(&total)
	var messages []model.Message
	err := r.db.Where("conversation_id = ?", convID).
		Order("created_at DESC").
		Offset((page-1)*pageSize).Limit(pageSize).
		Find(&messages).Error
	return messages, total, err
}

func (r *MessageRepository) Send(senderID, convID int64, body string) (*model.Message, error) {
	msg := &model.Message{
		ConversationID: convID,
		SenderID:       senderID,
		Body:           body,
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Conversation{}).Where("id = ?", convID).Update("updated_at", msg.CreatedAt).Error; err != nil {
			return err
		}
		return tx.Model(&model.ConversationParticipant{}).
			Where("conversation_id = ? AND user_id != ?", convID, senderID).
			UpdateColumn("unread_count", gorm.Expr("unread_count + 1")).Error
	})
	return msg, err
}

func (r *MessageRepository) IsParticipant(userID, convID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.ConversationParticipant{}).
		Where("user_id = ? AND conversation_id = ?", userID, convID).
		Count(&count).Error
	return count > 0, err
}

func (r *MessageRepository) UpdateLastRead(userID, convID int64) error {
	return r.db.Model(&model.ConversationParticipant{}).
		Where("user_id = ? AND conversation_id = ?", userID, convID).
		Updates(map[string]interface{}{
			"last_read_at": gorm.Expr("NOW()"),
			"unread_count": 0,
		}).Error
}

func (r *MessageRepository) DeleteMessage(msgID, userID int64) error {
	return r.db.Model(&model.Message{}).Where("id = ? AND sender_id = ?", msgID, userID).
		Update("body", "[message deleted]").Error
}

func (r *MessageRepository) LeaveConversation(convID, userID int64) error {
	return r.db.Where("conversation_id = ? AND user_id = ?", convID, userID).
		Delete(&model.ConversationParticipant{}).Error
}
