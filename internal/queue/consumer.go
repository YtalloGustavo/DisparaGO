package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"disparago/internal/platform/redisclient"
)

type Consumer interface {
	PopCampaignMessage(ctx context.Context) (string, error)
	RequeueCampaignMessage(ctx context.Context, payload string) error
}

type RedisConsumer struct {
	redis     *redisclient.Client
	queueName string
	timeout   time.Duration
}

func NewConsumer(redis *redisclient.Client, queueName string) *RedisConsumer {
	return &RedisConsumer{
		redis:     redis,
		queueName: queueName,
		timeout:   5 * time.Second,
	}
}

func (c *RedisConsumer) PopCampaignMessage(ctx context.Context) (string, error) {
	result, err := c.redis.Redis.BLPop(ctx, c.timeout, c.queueName).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("pop campaign message: %w", err)
	}

	if len(result) < 2 {
		return "", nil
	}

	return result[1], nil
}

func (c *RedisConsumer) RequeueCampaignMessage(ctx context.Context, payload string) error {
	if err := c.redis.Redis.RPush(ctx, c.queueName, payload).Err(); err != nil {
		return fmt.Errorf("requeue campaign message: %w", err)
	}

	return nil
}
