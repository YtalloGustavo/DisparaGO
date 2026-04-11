package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"disparago/internal/config"
)

type Client struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*Client, error) {
	pool, err := pgxpool.New(ctx, cfg.Postgres.URL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Client{Pool: pool}, nil
}

func (c *Client) Close(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		c.Pool.Close()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}
