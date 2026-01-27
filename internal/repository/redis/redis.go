package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/insider-one/notification-service/internal/config"
)

// Client wraps the Redis client
type Client struct {
	client *redis.Client
}

// New creates a new Redis client
func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opt.MaxRetries = cfg.MaxRetries
	opt.PoolSize = cfg.PoolSize
	opt.MinIdleConns = cfg.MinIdleConns

	client := redis.NewClient(opt)

	// Test the connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Client{client: client}, nil
}

// Close closes the Redis client
func (c *Client) Close() error {
	return c.client.Close()
}

// Health checks the Redis connection health
func (c *Client) Health(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// GetClient returns the underlying Redis client
func (c *Client) GetClient() *redis.Client {
	return c.client
}
