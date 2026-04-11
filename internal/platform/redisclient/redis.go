package redisclient

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"disparago/internal/config"
)

type Client struct {
	Redis *redis.Client
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	opts, err := redis.ParseURL(cfg.Redis.URL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Client{Redis: client}, nil
}

func (c *Client) Close() error {
	return c.Redis.Close()
}
