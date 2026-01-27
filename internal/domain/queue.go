package domain

import (
	"context"

	"github.com/google/uuid"
)

// QueueItem represents an item in the notification queue
type QueueItem struct {
	NotificationID uuid.UUID `json:"notification_id"`
	Channel        Channel   `json:"channel"`
	Priority       Priority  `json:"priority"`
	RetryCount     int       `json:"retry_count"`
}

// Queue defines the interface for the notification queue
type Queue interface {
	// Enqueue adds a notification to the queue
	Enqueue(ctx context.Context, item *QueueItem) error

	// EnqueueBatch adds multiple notifications to the queue
	EnqueueBatch(ctx context.Context, items []*QueueItem) error

	// Dequeue removes and returns the next item from the queue for a channel
	Dequeue(ctx context.Context, channel Channel) (*QueueItem, error)

	// GetQueueDepth returns the number of items in the queue for a channel
	GetQueueDepth(ctx context.Context, channel Channel) (int64, error)

	// GetAllQueueDepths returns queue depths for all channels
	GetAllQueueDepths(ctx context.Context) (map[Channel]int64, error)
}

// RateLimiter defines the interface for rate limiting
type RateLimiter interface {
	// Allow checks if a request is allowed under the rate limit
	Allow(ctx context.Context, channel Channel) (bool, error)

	// Wait blocks until a request is allowed
	Wait(ctx context.Context, channel Channel) error

	// GetCurrentRate returns the current rate for a channel
	GetCurrentRate(ctx context.Context, channel Channel) (int64, error)
}
