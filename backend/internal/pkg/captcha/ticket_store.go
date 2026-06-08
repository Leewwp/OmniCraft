package captcha

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrTicketInvalid          = errors.New("captcha ticket is invalid or expired")
	ErrTicketStoreUnavailable = errors.New("captcha ticket store is unavailable")
)

const defaultTicketTTLSec = 300

type TicketStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewTicketStore(rdb *redis.Client, ttlSec int) *TicketStore {
	if ttlSec <= 0 {
		ttlSec = defaultTicketTTLSec
	}
	return &TicketStore{
		rdb: rdb,
		ttl: time.Duration(ttlSec) * time.Second,
	}
}

func (s *TicketStore) Issue(ctx context.Context) (string, error) {
	if s == nil || s.rdb == nil {
		return "", ErrTicketStoreUnavailable
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate captcha ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	if err := s.rdb.Set(ctx, ticketKey(ticket), "verified", s.ttl).Err(); err != nil {
		return "", fmt.Errorf("failed to store captcha ticket: %w", err)
	}
	return ticket, nil
}

var consumeTicketScript = redis.NewScript(`
local val = redis.call("GET", KEYS[1])
if not val then
  return 0
end
redis.call("DEL", KEYS[1])
return 1
`)

func (s *TicketStore) Consume(ctx context.Context, ticket string) error {
	if s == nil || s.rdb == nil {
		return ErrTicketStoreUnavailable
	}
	ticket = strings.TrimSpace(ticket)
	if ticket == "" {
		return ErrTicketInvalid
	}

	consumed, err := consumeTicketScript.Run(ctx, s.rdb, []string{ticketKey(ticket)}).Int()
	if err != nil {
		return fmt.Errorf("failed to consume captcha ticket: %w", err)
	}
	if consumed != 1 {
		return ErrTicketInvalid
	}
	return nil
}

func ticketKey(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return "captcha:ticket:" + hex.EncodeToString(sum[:])
}

type TicketAwareVerifier struct {
	provider         string
	providerVerifier CaptchaVerifier
	tickets          *TicketStore
}

func NewTicketAwareVerifier(provider string, providerVerifier CaptchaVerifier, tickets *TicketStore) CaptchaVerifier {
	return &TicketAwareVerifier{
		provider:         strings.TrimSpace(strings.ToLower(provider)),
		providerVerifier: providerVerifier,
		tickets:          tickets,
	}
}

func (v *TicketAwareVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if v != nil && v.provider == "aliyun_v2" {
		return v.tickets.Consume(ctx, token)
	}
	if v != nil && v.providerVerifier != nil {
		return v.providerVerifier.Verify(ctx, token, remoteIP)
	}
	return nil
}
