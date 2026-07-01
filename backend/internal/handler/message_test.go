package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/model"
)

func TestMessageColdStartAllowsFirstMessage(t *testing.T) {
	tests := []struct {
		name       string
		senderID   int64
		recipient  int64
		wantStatus int
	}{
		{
			name:       "first message in a new conversation is allowed",
			senderID:   1,
			recipient:  2,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupMessageColdStartRouter(t)

			rec := postMessageForColdStart(t, router, tt.senderID, tt.recipient, "hello")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestMessageColdStartRejectsSecondUnansweredMessage(t *testing.T) {
	tests := []struct {
		name              string
		senderID          int64
		recipient         int64
		wantStatus        int
		wantCode          string
		wantMessage       string
		wantMessageCount  int64
		firstMessageText  string
		secondMessageText string
	}{
		{
			name:              "same sender cannot send again before recipient replies",
			senderID:          1,
			recipient:         2,
			wantStatus:        http.StatusForbidden,
			wantCode:          "DM_REPLY_REQUIRED",
			wantMessage:       "对方尚未回复，请等待回复后再发送新消息",
			wantMessageCount:  1,
			firstMessageText:  "hello",
			secondMessageText: "still there?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, db := setupMessageColdStartRouter(t)
			first := postMessageForColdStart(t, router, tt.senderID, tt.recipient, tt.firstMessageText)
			if first.Code != http.StatusCreated {
				t.Fatalf("first message status = %d, want 201; body = %s", first.Code, first.Body.String())
			}

			rec := postMessageForColdStart(t, router, tt.senderID, tt.recipient, tt.secondMessageText)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			var payload struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error response: %v; body = %s", err, rec.Body.String())
			}
			if payload.Code != tt.wantCode || payload.Message != tt.wantMessage {
				t.Fatalf("response = %#v, want code %q message %q", payload, tt.wantCode, tt.wantMessage)
			}

			var count int64
			if err := db.Model(&model.Message{}).Count(&count).Error; err != nil {
				t.Fatalf("count messages: %v", err)
			}
			if count != tt.wantMessageCount {
				t.Fatalf("message count = %d, want %d", count, tt.wantMessageCount)
			}
		})
	}
}

func TestMessageColdStartAllowsRecipientReply(t *testing.T) {
	tests := []struct {
		name       string
		senderID   int64
		recipient  int64
		wantStatus int
	}{
		{
			name:       "recipient can reply to the first message",
			senderID:   2,
			recipient:  1,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupMessageColdStartRouter(t)
			first := postMessageForColdStart(t, router, 1, 2, "hello")
			if first.Code != http.StatusCreated {
				t.Fatalf("first message status = %d, want 201; body = %s", first.Code, first.Body.String())
			}

			rec := postMessageForColdStart(t, router, tt.senderID, tt.recipient, "reply")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestMessageColdStartAllowsSenderAfterRecipientHasReplied(t *testing.T) {
	tests := []struct {
		name       string
		senderID   int64
		recipient  int64
		wantStatus int
	}{
		{
			name:       "original sender can send after recipient has replied",
			senderID:   1,
			recipient:  2,
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := setupMessageColdStartRouter(t)
			first := postMessageForColdStart(t, router, 1, 2, "hello")
			if first.Code != http.StatusCreated {
				t.Fatalf("first message status = %d, want 201; body = %s", first.Code, first.Body.String())
			}
			reply := postMessageForColdStart(t, router, 2, 1, "reply")
			if reply.Code != http.StatusCreated {
				t.Fatalf("reply status = %d, want 201; body = %s", reply.Code, reply.Body.String())
			}

			rec := postMessageForColdStart(t, router, tt.senderID, tt.recipient, "thanks")

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestMessageColdStartRejectsAfterSenderLeavesAndReopensConversation(t *testing.T) {
	router, db := setupMessageColdStartRouter(t)
	first := postMessageForColdStart(t, router, 1, 2, "hello")
	if first.Code != http.StatusCreated {
		t.Fatalf("first message status = %d, want 201; body = %s", first.Code, first.Body.String())
	}

	leftAt := time.Now()
	if err := db.Model(&model.ConversationParticipant{}).
		Where("user_id = ? AND left_at IS NULL", int64(1)).
		Update("left_at", leftAt).Error; err != nil {
		t.Fatalf("mark sender left: %v", err)
	}

	rec := postMessageForColdStart(t, router, 1, 2, "reopened hello")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v; body = %s", err, rec.Body.String())
	}
	if payload.Code != "DM_REPLY_REQUIRED" || payload.Message != "对方尚未回复，请等待回复后再发送新消息" {
		t.Fatalf("response = %#v, want DM_REPLY_REQUIRED exact message", payload)
	}

	var count int64
	if err := db.Model(&model.Message{}).Count(&count).Error; err != nil {
		t.Fatalf("count messages: %v", err)
	}
	if count != 1 {
		t.Fatalf("message count = %d, want 1", count)
	}
}

func TestMessageAPIContract(t *testing.T) {
	t.Run("list conversations returns message center DTO envelope", func(t *testing.T) {
		router, db := setupMessageColdStartRouter(t)
		createdAt := time.Date(2026, 6, 30, 12, 1, 0, 0, time.UTC)
		updatedAt := time.Date(2026, 6, 30, 12, 2, 0, 0, time.UTC)
		seedMessageConversation(t, db, createdAt, updatedAt)

		rec := getMessageEndpoint(t, router, 1, "/api/v1/messages")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Conversations []struct {
				ID           int64 `json:"id"`
				Participants []struct {
					ID        int64  `json:"id"`
					Username  string `json:"username"`
					AvatarURL string `json:"avatar_url"`
				} `json:"participants"`
				LastMessage struct {
					ID        int64  `json:"id"`
					Text      string `json:"text"`
					SenderID  int64  `json:"sender_id"`
					CreatedAt string `json:"created_at"`
				} `json:"last_message"`
				UnreadCount int    `json:"unread_count"`
				UpdatedAt   string `json:"updated_at"`
			} `json:"conversations"`
			Page     int `json:"page"`
			PageSize int `json:"page_size"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
		}
		if payload.Page != 1 || payload.PageSize != 20 {
			t.Fatalf("pagination = page %d page_size %d, want 1/20", payload.Page, payload.PageSize)
		}
		if len(payload.Conversations) != 1 {
			t.Fatalf("conversation count = %d, want 1; payload = %#v", len(payload.Conversations), payload)
		}
		conv := payload.Conversations[0]
		if conv.ID != 1 {
			t.Fatalf("conversation id = %d, want 1", conv.ID)
		}
		if len(conv.Participants) != 1 {
			t.Fatalf("participant count = %d, want 1; participants = %#v", len(conv.Participants), conv.Participants)
		}
		if got := conv.Participants[0]; got.ID != 2 || got.Username != "bob" || got.AvatarURL != "" {
			t.Fatalf("participant = %#v, want bob DTO", got)
		}
		if conv.LastMessage.ID != 10 || conv.LastMessage.Text != "hello" || conv.LastMessage.SenderID != 1 {
			t.Fatalf("last_message = %#v, want id/text/sender", conv.LastMessage)
		}
		if conv.LastMessage.CreatedAt != createdAt.Format(time.RFC3339Nano) {
			t.Fatalf("last_message.created_at = %q, want %q", conv.LastMessage.CreatedAt, createdAt.Format(time.RFC3339Nano))
		}
		if conv.UnreadCount != 1 {
			t.Fatalf("unread_count = %d, want 1", conv.UnreadCount)
		}
		if conv.UpdatedAt != updatedAt.Format(time.RFC3339Nano) {
			t.Fatalf("updated_at = %q, want %q", conv.UpdatedAt, updatedAt.Format(time.RFC3339Nano))
		}
	})

	t.Run("list messages returns text and body during compatibility window", func(t *testing.T) {
		router, db := setupMessageColdStartRouter(t)
		createdAt := time.Date(2026, 6, 30, 12, 1, 0, 0, time.UTC)
		seedMessageConversation(t, db, createdAt, time.Date(2026, 6, 30, 12, 2, 0, 0, time.UTC))

		rec := getMessageEndpoint(t, router, 1, "/api/v1/messages/1")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
		}
		var payload struct {
			Messages []struct {
				ID        int64  `json:"id"`
				SenderID  int64  `json:"sender_id"`
				Text      string `json:"text"`
				Body      string `json:"body"`
				CreatedAt string `json:"created_at"`
			} `json:"messages"`
			Total int64 `json:"total"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response: %v; body = %s", err, rec.Body.String())
		}
		if payload.Total != 1 || len(payload.Messages) != 1 {
			t.Fatalf("message total/count = %d/%d, want 1/1", payload.Total, len(payload.Messages))
		}
		msg := payload.Messages[0]
		if msg.ID != 10 || msg.SenderID != 1 || msg.Text != "hello" || msg.Body != "hello" {
			t.Fatalf("message = %#v, want text and body aliases", msg)
		}
		if msg.CreatedAt != createdAt.Format(time.RFC3339Nano) {
			t.Fatalf("created_at = %q, want %q", msg.CreatedAt, createdAt.Format(time.RFC3339Nano))
		}
	})
}

func setupMessageColdStartRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Conversation{}, &model.ConversationParticipant{}, &model.Message{}); err != nil {
		t.Fatalf("migrate messages: %v", err)
	}

	handler := NewMessageHandler(db)
	router := gin.New()
	router.POST("/api/v1/messages", func(c *gin.Context) {
		if rawUserID := c.GetHeader("X-Test-User-ID"); rawUserID != "" {
			userID, err := strconv.ParseInt(rawUserID, 10, 64)
			if err != nil {
				t.Fatalf("parse test user id %q: %v", rawUserID, err)
			}
			c.Set(middleware.UserIDKey, userID)
		}
		handler.SendMessage(c)
	})
	router.GET("/api/v1/messages", func(c *gin.Context) {
		setTestUserID(t, c)
		handler.ListConversations(c)
	})
	router.GET("/api/v1/messages/:id", func(c *gin.Context) {
		setTestUserID(t, c)
		handler.ListMessages(c)
	})

	return router, db
}

func postMessageForColdStart(t *testing.T, router *gin.Engine, senderID, recipientID int64, text string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]interface{}{
		"recipient_id": recipientID,
		"text":         text,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(senderID, 10))
	router.ServeHTTP(rec, req)
	return rec
}

func getMessageEndpoint(t *testing.T, router *gin.Engine, userID int64, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	router.ServeHTTP(rec, req)
	return rec
}

func setTestUserID(t *testing.T, c *gin.Context) {
	t.Helper()

	rawUserID := c.GetHeader("X-Test-User-ID")
	if rawUserID == "" {
		return
	}
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil {
		t.Fatalf("parse test user id %q: %v", rawUserID, err)
	}
	c.Set(middleware.UserIDKey, userID)
}

func seedMessageConversation(t *testing.T, db *gorm.DB, createdAt, updatedAt time.Time) {
	t.Helper()

	users := []model.User{
		{ID: 1, Email: "alice@example.test", PasswordHash: "hash", Username: "alice"},
		{ID: 2, Email: "bob@example.test", PasswordHash: "hash", Username: "bob"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	conversation := model.Conversation{ID: 1, CreatedAt: createdAt.Add(-time.Minute), UpdatedAt: updatedAt}
	if err := db.Create(&conversation).Error; err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	participants := []model.ConversationParticipant{
		{ConversationID: 1, UserID: 1, UnreadCount: 1},
		{ConversationID: 1, UserID: 2},
	}
	if err := db.Create(&participants).Error; err != nil {
		t.Fatalf("seed participants: %v", err)
	}
	message := model.Message{ID: 10, ConversationID: 1, SenderID: 1, Body: "hello", CreatedAt: createdAt}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("seed message: %v", err)
	}
}
