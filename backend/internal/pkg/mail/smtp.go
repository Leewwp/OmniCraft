package mail

import (
	"context"
	"fmt"
	"net/smtp"

	"omnicraft/backend/config"
)

type SMTPSender struct {
	host     string
	port     int
	user     string
	password string
	from     string
}

func NewSMTPSender(cfg config.SMTPConfig) *SMTPSender {
	return &SMTPSender{
		host:     cfg.Host,
		port:     cfg.Port,
		user:     cfg.User,
		password: cfg.Password,
		from:     cfg.FromAddress,
	}
}

func (s *SMTPSender) SendVerification(ctx context.Context, to, link string) error {
	subject := "Verify your email"
	body := fmt.Sprintf("Click the following link to verify your email: %s", link)
	return s.sendMail(to, subject, body)
}

func (s *SMTPSender) SendPasswordReset(ctx context.Context, to, link string) error {
	subject := "Reset your password"
	body := fmt.Sprintf("Click the following link to reset your password: %s", link)
	return s.sendMail(to, subject, body)
}

func (s *SMTPSender) sendMail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	from := s.from
	if from == "" {
		from = s.user
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body,
	)

	auth := smtp.PlainAuth("", s.user, s.password, s.host)
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
