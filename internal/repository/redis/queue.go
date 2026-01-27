package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/insider-one/notification-service/internal/domain"
)

const (
	queueKeyPrefix = "notification:queue:"
)

// Queue implements domain.Queue using Redis Sorted Sets
type Queue struct {
	client *Client
}

// NewQueue creates a new Queue
func NewQueue(client *Client) *Queue {
	return &Queue{client: client}
}

// queueKey returns the Redis key for a channel's queue
func queueKey(channel domain.Channel) string {
	return queueKeyPrefix + string(channel)
}

// Enqueue adds a notification to the queue
func (q *Queue) Enqueue(ctx context.Context, item *domain.QueueItem) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal queue item: %w", err)
	}

	// Calculate score: priority weight + timestamp for ordering
	score := float64(item.Priority.Weight()) + float64(time.Now().UnixNano())/1e18

	key := queueKey(item.Channel)
	if err := q.client.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: string(data),
	}).Err(); err != nil {
		return fmt.Errorf("failed to enqueue item: %w", err)
	}

	return nil
}

// EnqueueBatch adds multiple notifications to the queue
func (q *Queue) EnqueueBatch(ctx context.Context, items []*domain.QueueItem) error {
	if len(items) == 0 {
		return nil
	}

	// Group items by channel
	channelItems := make(map[domain.Channel][]redis.Z)
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal queue item: %w", err)
		}

		score := float64(item.Priority.Weight()) + float64(time.Now().UnixNano())/1e18
		channelItems[item.Channel] = append(channelItems[item.Channel], redis.Z{
			Score:  score,
			Member: string(data),
		})
	}

	// Use pipeline for batch insert
	pipe := q.client.client.Pipeline()
	for channel, zItems := range channelItems {
		pipe.ZAdd(ctx, queueKey(channel), zItems...)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to enqueue batch: %w", err)
	}

	return nil
}

// Dequeue removes and returns the next item from the queue
func (q *Queue) Dequeue(ctx context.Context, channel domain.Channel) (*domain.QueueItem, error) {
	key := queueKey(channel)

	// Use ZPOPMIN to atomically get and remove the lowest score item
	results, err := q.client.client.ZPopMin(ctx, key, 1).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Queue is empty
		}
		return nil, fmt.Errorf("failed to dequeue item: %w", err)
	}

	if len(results) == 0 {
		return nil, nil // Queue is empty
	}

	var item domain.QueueItem
	if err := json.Unmarshal([]byte(results[0].Member.(string)), &item); err != nil {
		return nil, fmt.Errorf("failed to unmarshal queue item: %w", err)
	}

	return &item, nil
}

// GetQueueDepth returns the number of items in the queue for a channel
func (q *Queue) GetQueueDepth(ctx context.Context, channel domain.Channel) (int64, error) {
	key := queueKey(channel)
	count, err := q.client.client.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get queue depth: %w", err)
	}
	return count, nil
}

// GetAllQueueDepths returns queue depths for all channels
func (q *Queue) GetAllQueueDepths(ctx context.Context) (map[domain.Channel]int64, error) {
	channels := []domain.Channel{domain.ChannelSMS, domain.ChannelEmail, domain.ChannelPush}
	depths := make(map[domain.Channel]int64)

	pipe := q.client.client.Pipeline()
	cmds := make(map[domain.Channel]*redis.IntCmd)

	for _, channel := range channels {
		cmds[channel] = pipe.ZCard(ctx, queueKey(channel))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("failed to get queue depths: %w", err)
	}

	for channel, cmd := range cmds {
		depths[channel] = cmd.Val()
	}

	return depths, nil
}
