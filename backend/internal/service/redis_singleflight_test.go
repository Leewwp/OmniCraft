package service

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

/* #728 生产去重路径：Redis SET NX PX（跨进程有效——两个客户端实例模拟
 * server 与 worker 进程；TTL 兜底崩溃持有者）。 */

func TestRedisSingleflightCrossClient(t *testing.T) {
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	server := NewRedisSingleflight(redis.NewClient(&redis.Options{Addr: addr}))
	worker := NewRedisSingleflight(redis.NewClient(&redis.Options{Addr: addr}))
	ctx := context.Background()

	ok1, err := server.Acquire(ctx, "k1", time.Minute)
	if err != nil || !ok1 {
		t.Fatalf("server acquire: %v %v", ok1, err)
	}
	// 另一进程（worker）同键必须拿不到——跨进程去重成立。
	ok2, err := worker.Acquire(ctx, "k1", time.Minute)
	if err != nil {
		t.Fatalf("worker acquire err: %v", err)
	}
	if ok2 {
		t.Fatal("worker must NOT acquire while server holds the lease")
	}
	// 释放后可再取。
	if err := server.Release(ctx, "k1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	ok3, err := worker.Acquire(ctx, "k1", time.Minute)
	if err != nil || !ok3 {
		t.Fatalf("worker reacquire after release: %v %v", ok3, err)
	}
	// TTL 兜底：持有者崩溃后租约过期。
	mr.FastForward(2 * time.Minute)
	ok4, err := server.Acquire(ctx, "k1", time.Minute)
	if err != nil || !ok4 {
		t.Fatalf("server acquire after TTL expiry: %v %v", ok4, err)
	}
}
