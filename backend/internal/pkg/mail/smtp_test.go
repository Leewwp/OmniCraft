package mail

import (
	"bytes"
	"net/smtp"
	"testing"
)

func TestSMTPSenderUsesImplicitTLSForPort465(t *testing.T) {
	origPlain := sendSMTPPlain
	origImplicitTLS := sendSMTPImplicitTLS
	t.Cleanup(func() {
		sendSMTPPlain = origPlain
		sendSMTPImplicitTLS = origImplicitTLS
	})

	var calledTLS bool
	sendSMTPPlain = func(string, smtp.Auth, string, []string, []byte) error {
		t.Fatal("plain SMTP path should not be used for port 465")
		return nil
	}
	sendSMTPImplicitTLS = func(addr, serverName string, auth smtp.Auth, from string, to []string, msg []byte) error {
		calledTLS = true
		if addr != "smtpdm-shenzhen.aliyuncs.com:465" {
			t.Fatalf("addr = %q", addr)
		}
		if serverName != "smtpdm-shenzhen.aliyuncs.com" {
			t.Fatalf("serverName = %q", serverName)
		}
		if from != "leeppp@leeppp.online" {
			t.Fatalf("from = %q", from)
		}
		if len(to) != 1 || to[0] != "user@example.com" {
			t.Fatalf("to = %#v", to)
		}
		if !bytes.Contains(msg, []byte("Subject: Verify")) {
			t.Fatalf("message missing subject: %s", msg)
		}
		if auth == nil {
			t.Fatal("auth is nil")
		}
		return nil
	}

	sender := &SMTPSender{
		host:     "smtpdm-shenzhen.aliyuncs.com",
		port:     465,
		user:     "leeppp@leeppp.online",
		password: "secret",
		from:     "leeppp@leeppp.online",
	}

	if err := sender.sendMail("user@example.com", "Verify", "body"); err != nil {
		t.Fatalf("sendMail: %v", err)
	}
	if !calledTLS {
		t.Fatal("implicit TLS SMTP path was not called")
	}
}

func TestSMTPSenderUsesPlainSMTPForNon465Ports(t *testing.T) {
	origPlain := sendSMTPPlain
	origImplicitTLS := sendSMTPImplicitTLS
	t.Cleanup(func() {
		sendSMTPPlain = origPlain
		sendSMTPImplicitTLS = origImplicitTLS
	})

	var calledPlain bool
	sendSMTPPlain = func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
		calledPlain = true
		if addr != "smtp.example.com:587" {
			t.Fatalf("addr = %q", addr)
		}
		return nil
	}
	sendSMTPImplicitTLS = func(string, string, smtp.Auth, string, []string, []byte) error {
		t.Fatal("implicit TLS path should not be used for port 587")
		return nil
	}

	sender := &SMTPSender{
		host:     "smtp.example.com",
		port:     587,
		user:     "user@example.com",
		password: "secret",
	}

	if err := sender.sendMail("user@example.com", "Subject", "body"); err != nil {
		t.Fatalf("sendMail: %v", err)
	}
	if !calledPlain {
		t.Fatal("plain SMTP path was not called")
	}
}
