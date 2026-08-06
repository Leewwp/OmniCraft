package mail

import (
	"context"
	"log/slog"
)

type MailSender interface {
	SendVerification(ctx context.Context, to, link string) error
	SendPasswordReset(ctx context.Context, to, link string) error
}

type LoggerSender struct{}

func NewLoggerSender() *LoggerSender {
	return &LoggerSender{}
}

func (s *LoggerSender) SendVerification(ctx context.Context, to, link string) error {
	slog.Info("mail event", "event", "verification_email")
	return nil
}

func (s *LoggerSender) SendPasswordReset(ctx context.Context, to, link string) error {
	slog.Info("mail event", "event", "password_reset_email")
	return nil
}

func (s *LoggerSender) SendFeedbackUpdate(ctx context.Context, to, subject, body string) error {
	slog.Info("mail event", "event", "feedback_update_email")
	return nil
}
