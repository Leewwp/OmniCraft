package service

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSingleflight is the #728 cross-process generation dedup seam: server
// and worker are separate processes, so merging duplicate generations must
// not rely on in-process singleflight. Production uses Redis SET NX PX;
// tests substitute an in-memory fake.
type RedisSingleflight interface {
	// Acquire tries to take the generation lease; false = someone else holds it.
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Release drops the lease (owner only — best effort, TTL bounds it).
	Release(ctx context.Context, key string) error
}

type redisSingleflight struct{ rdb *redis.Client }

func NewRedisSingleflight(rdb *redis.Client) RedisSingleflight {
	return &redisSingleflight{rdb: rdb}
}

func (s *redisSingleflight) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if s.rdb == nil {
		return false, errors.New("redis unavailable")
	}
	ok, err := s.rdb.SetNX(ctx, "guidegen:"+key, "1", ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *redisSingleflight) Release(ctx context.Context, key string) error {
	if s.rdb == nil {
		return errors.New("redis unavailable")
	}
	return s.rdb.Del(ctx, "guidegen:"+key).Err()
}
