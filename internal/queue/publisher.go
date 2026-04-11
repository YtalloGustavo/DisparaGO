package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"disparago/internal/platform/redisclient"
)

type CampaignMessageJob struct {
	MessageID    string `json:"message_id"`
	CampaignID   string `json:"campaign_id"`
	InstanceID   string `json:"instance_id"`
	Recipient    string `json:"recipient"`
	Message      string `json:"message"`
	AttemptCount int    `json:"attempt_count"`
}

type Publisher struct {
	redis     *redisclient.Client
	queueName string
}

func NewPublisher(redis *redisclient.Client, queueName string) *Publisher {
	return &Publisher{
		redis:     redis,
		queueName: queueName,
	}
}

func (p *Publisher) PublishCampaignMessages(ctx context.Context, jobs []CampaignMessageJob) error {
	if len(jobs) == 0 {
		return nil
	}

	payloads := make([]interface{}, 0, len(jobs))
	for _, job := range jobs {
		raw, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("marshal campaign message job: %w", err)
		}
		payloads = append(payloads, raw)
	}

	if err := p.redis.Redis.RPush(ctx, p.queueName, payloads...).Err(); err != nil {
		return fmt.Errorf("publish campaign messages: %w", err)
	}

	return nil
}
