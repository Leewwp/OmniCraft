package worker

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"omnicraft/backend/internal/pkg/queue"
	"omnicraft/backend/internal/repository"
	"omnicraft/backend/internal/service"
	"omnicraft/backend/internal/testutil"
)

// TestRelayWorkerGracefulShutdown proves the relay loop exits promptly when its
// context is cancelled (SIGTERM path of cmd/worker).
func TestRelayWorkerGracefulShutdown(t *testing.T) {
	db := testutil.OpenEphemeralPostgres(t)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "070_outbox_inbox.sql"))
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	broker := queue.NewRedisStreamBroker(rdb, &queue.QueueConfig{MaxLen: 100000, MaxAttempts: 3, RetryBackoffSec: []int{1}})
	defer broker.Stop()

	relay := NewRelayWorker(service.NewRelayService(repository.NewOutboxRepository(db), broker, &queue.QueueConfig{RetryBackoffSec: []int{1}}), 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- relay.Start(ctx) }()

	// Let the loop run a few cycles, then cancel.
	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled, "relay must return the cancelled context error")
	case <-time.After(3 * time.Second):
		t.Fatal("relay worker did not stop after context cancellation")
	}
}

func TestWorkerManagerGracefulStop(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cfg := &queue.QueueConfig{Enabled: true, MaxAttempts: 3, RetryBackoffSec: []int{0, 0}, MaxLen: 100000}
	broker := queue.NewRedisStreamBroker(rdb, cfg)
	mgr := NewWorkerManager(broker)

	var handled int
	var mu sync.Mutex
	mgr.Register("test.graceful", "omnicraft-graceful", func(ctx context.Context, msg queue.Message) error {
		mu.Lock()
		handled++
		mu.Unlock()
		return nil
	})

	require.NoError(t, mgr.Start(context.Background()))
	require.NoError(t, broker.Publish(context.Background(), "test.graceful", []byte("one")))

	deadline := time.Now().Add(5 * time.Second)
	for {
		mu.Lock()
		n := handled
		mu.Unlock()
		if n >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first message was never handled")
		}
		time.Sleep(50 * time.Millisecond)
	}

	mgr.Stop()

	// Messages published after Stop must not reach the handler.
	require.NoError(t, broker.Publish(context.Background(), "test.graceful", []byte("two")))
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, handled, "handler must not run after graceful stop")
}
