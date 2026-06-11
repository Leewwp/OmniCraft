package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrUploadGrantInvalid = errors.New("upload grant invalid or expired")
var ErrUploadGrantUnavailable = errors.New("upload grant store unavailable")

var uploadGrantEntropyReader io.Reader = rand.Reader

type UploadGrant struct {
	ID       string `json:"id"`
	UserID   int64  `json:"user_id"`
	Purpose  string `json:"purpose"`
	OSSKey   string `json:"oss_key"`
	FileType string `json:"file_type"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

type UploadGrantService struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewUploadGrantService(rdb *redis.Client, ttl time.Duration) *UploadGrantService {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &UploadGrantService{rdb: rdb, ttl: ttl}
}

func (s *UploadGrantService) Issue(ctx context.Context, grant UploadGrant) (*UploadGrant, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrUploadGrantUnavailable
	}
	grantID, err := randomGrantID()
	if err != nil {
		return nil, err
	}
	grant.ID = grantID
	raw, err := json.Marshal(grant)
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, uploadGrantKey(grant.ID), raw, s.ttl).Err(); err != nil {
		return nil, err
	}
	return &grant, nil
}

func (s *UploadGrantService) Consume(ctx context.Context, id string, userID int64, purpose string) (*UploadGrant, error) {
	if s == nil || s.rdb == nil {
		return nil, ErrUploadGrantUnavailable
	}
	if id == "" {
		return nil, ErrUploadGrantInvalid
	}
	key := uploadGrantKey(id)
	var consumed *UploadGrant
	err := s.rdb.Watch(ctx, func(tx *redis.Tx) error {
		raw, err := tx.Get(ctx, key).Bytes()
		if err == redis.Nil {
			return ErrUploadGrantInvalid
		}
		if err != nil {
			return err
		}
		var grant UploadGrant
		if err := json.Unmarshal(raw, &grant); err != nil {
			return ErrUploadGrantInvalid
		}
		if grant.UserID != userID || grant.Purpose != purpose {
			return ErrUploadGrantInvalid
		}
		if _, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, key)
			return nil
		}); err != nil {
			return err
		}
		consumed = &grant
		return nil
	}, key)
	if err == redis.TxFailedErr {
		return nil, ErrUploadGrantInvalid
	}
	if err != nil {
		return nil, err
	}
	return consumed, nil
}

func (s *UploadGrantService) Restore(ctx context.Context, grant UploadGrant) error {
	if s == nil || s.rdb == nil {
		return ErrUploadGrantUnavailable
	}
	if grant.ID == "" {
		return ErrUploadGrantInvalid
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		return err
	}
	return s.rdb.SetNX(ctx, uploadGrantKey(grant.ID), raw, s.ttl).Err()
}

func uploadGrantKey(id string) string {
	return fmt.Sprintf("upload:grant:%s", id)
}

func randomGrantID() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(uploadGrantEntropyReader, raw); err != nil {
		return "", fmt.Errorf("%w: entropy unavailable", ErrUploadGrantUnavailable)
	}
	return hex.EncodeToString(raw), nil
}
