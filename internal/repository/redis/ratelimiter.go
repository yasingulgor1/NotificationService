package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/insider-one/notification-service/internal/domain"
)

const (
	rateLimitKeyPrefix = "ratelimit:"
	rateLimitWindow    = time.Second
)

// RateLimiter implements domain.RateLimiter using Redis
type RateLimiter struct {
	client      *Client
	limitPerSec int
}

// NewRateLimiter creates a new RateLimiter
func NewRateLimiter(client *Client, limitPerSec int) *RateLimiter {
	return &RateLimiter{
		client:      client,
		limitPerSec: limitPerSec,
	}
}

// rateLimitKey returns the Redis key for rate limiting
func rateLimitKey(channel domain.Channel) string {
	return rateLimitKeyPrefix + string(channel)
}

// Allow checks if a request is allowed under the rate limit using sliding window
func (r *RateLimiter) Allow(ctx context.Context, channel domain.Channel) (bool, error) {
	key := rateLimitKey(channel)
	now := time.Now()
	windowStart := now.Add(-rateLimitWindow)

	pipe := r.client.client.Pipeline()

	// Remove old entries outside the window
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current entries in the window
	countCmd := pipe.ZCard(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("failed to check rate limit: %w", err)
	}

	currentCount := countCmd.Val()
	if currentCount >= int64(r.limitPerSec) {
		return false, nil
	}

	// Add new entry with current timestamp as score
	if err := r.client.client.ZAdd(ctx, key, struct {
		Score  float64
		Member any
	}{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	}).Err(); err != nil {
		return false, fmt.Errorf("failed to record request: %w", err)
	}

	// Set expiry on the key
	r.client.client.Expire(ctx, key, 2*rateLimitWindow)

	return true, nil
}

// Wait blocks until a request is allowed
func (r *RateLimiter) Wait(ctx context.Context, channel domain.Channel) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			allowed, err := r.Allow(ctx, channel)
			if err != nil {
				return err
			}
			if allowed {
				return nil
			}
		}
	}
}

// GetCurrentRate returns the current rate for a channel
func (r *RateLimiter) GetCurrentRate(ctx context.Context, channel domain.Channel) (int64, error) {
	key := rateLimitKey(channel)
	now := time.Now()
	windowStart := now.Add(-rateLimitWindow)

	// Remove old entries and count
	pipe := r.client.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
	countCmd := pipe.ZCard(ctx, key)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, fmt.Errorf("failed to get current rate: %w", err)
	}

	return countCmd.Val(), nil
}
