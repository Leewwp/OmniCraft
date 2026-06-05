package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"

	"omnicraft/backend/config"
)

var (
	sendSMTPPlain       = smtp.SendMail
	sendSMTPImplicitTLS = sendMailImplicitTLS
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
	if s.port == 465 {
		return sendSMTPImplicitTLS(addr, s.host, auth, from, []string{to}, []byte(msg))
	}
	return sendSMTPPlain(addr, auth, from, []string{to}, []byte(msg))
}

func sendMailImplicitTLS(addr, serverName string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, serverName)
	if err != nil {
		return err
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
