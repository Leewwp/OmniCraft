package captcha

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestTicketStoreIssuesAndConsumesOnce(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewTicketStore(rdb, 120)

	ticket, err := store.Issue(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, ticket)

	require.NoError(t, store.Consume(context.Background(), ticket))
	require.ErrorIs(t, store.Consume(context.Background(), ticket), ErrTicketInvalid)
}

func TestTicketStoreRejectsMissingAndExpiredTickets(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewTicketStore(rdb, 1)

	require.ErrorIs(t, store.Consume(context.Background(), "missing-ticket"), ErrTicketInvalid)

	ticket, err := store.Issue(context.Background())
	require.NoError(t, err)
	mr.FastForward(2 * time.Second)

	require.ErrorIs(t, store.Consume(context.Background(), ticket), ErrTicketInvalid)
}
