package guestid

import (
	"strings"
	"testing"
)

const testSecret = "unit-test-guest-cookie-secret-0123456789abcdef"

func TestIssueVerifyRoundTrip(t *testing.T) {
	cookie, key1, err := Issue(testSecret)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	key2, err := Verify(testSecret, cookie)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if key1 == "" || key1 != key2 {
		t.Fatalf("device key round trip mismatch: %q vs %q", key1, key2)
	}
	if len(key1) != 64 || strings.ToLower(key1) != key1 {
		t.Fatalf("device key must be lowercase sha256 hex, got %q", key1)
	}
}

func TestIssueIsRandom(t *testing.T) {
	a, _, err := Issue(testSecret)
	if err != nil {
		t.Fatalf("issue a: %v", err)
	}
	b, _, err := Issue(testSecret)
	if err != nil {
		t.Fatalf("issue b: %v", err)
	}
	if a == b {
		t.Fatal("two issues must never collide")
	}
}

func TestVerifyRejectsTamperedSignature(t *testing.T) {
	cookie, _, err := Issue(testSecret)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("cookie shape: %q", cookie)
	}
	sig := parts[1]
	if sig[0] == '0' {
		sig = "1" + sig[1:]
	} else {
		sig = "0" + sig[1:]
	}
	if _, err := Verify(testSecret, parts[0]+"."+sig); err == nil {
		t.Fatal("tampered signature must be rejected")
	}
	// A flipped body byte must equally fail.
	body := parts[0]
	if body[0] == '0' {
		body = "1" + body[1:]
	} else {
		body = "0" + body[1:]
	}
	if _, err := Verify(testSecret, body+"."+parts[1]); err == nil {
		t.Fatal("tampered id must be rejected")
	}
}

func TestVerifyRejectsWrongSecretAndMalformed(t *testing.T) {
	cookie, _, err := Issue(testSecret)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := Verify("another-secret-0123456789abcdef-unit-test", cookie); err == nil {
		t.Fatal("wrong secret must be rejected")
	}
	for _, malformed := range []string{"", "no-signature", "a.b", "....", cookie + ".extra"} {
		if _, err := Verify(testSecret, malformed); err == nil {
			t.Fatalf("malformed cookie %q must be rejected", malformed)
		}
	}
}
