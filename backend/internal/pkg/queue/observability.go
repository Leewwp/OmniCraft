package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

type QueueStats struct {
	Topic        string `json:"topic"`
	Depth        int64  `json:"depth"`
	Consumed     int64  `json:"consumed_total"`
	Failed       int64  `json:"failed_total"`
	PendingCount int64  `json:"pending_count"`
}

func GetQueueStats(ctx context.Context, rdb *redis.Client, topics []string) ([]QueueStats, error) {
	var stats []QueueStats
	for _, topic := range topics {
		key := streamKey(topic)
		info, err := rdb.XInfoStream(ctx, key).Result()
		if err != nil {
			if err.Error() == "ERR no such key" || err.Error() == "NOGROUP" {
				stats = append(stats, QueueStats{Topic: topic})
				continue
			}
			return nil, fmt.Errorf("xinfo stream %s: %w", key, err)
		}
		s := QueueStats{
			Topic:  topic,
			Depth:  info.Length,
		}
		groups, err := rdb.XInfoGroups(ctx, key).Result()
		if err == nil && len(groups) > 0 {
			s.PendingCount = groups[0].Pending
			s.Consumed = int64(groups[0].Consumers)
		}
		stats = append(stats, s)
	}
	return stats, nil
}

func GetDLQStats(ctx context.Context, rdb *redis.Client) (int64, error) {
	length, err := rdb.XLen(ctx, "omnicraft:dead-letter").Result()
	if err != nil {
		return 0, err
	}
	return length, nil
}

func logQueueEvent(event, topic, msgID string, attempt int) {
	slog.Info("queue event",
		"event", event,
		"queue_name", topic,
		"msg_id", msgID,
		"attempt", attempt,
	)
}