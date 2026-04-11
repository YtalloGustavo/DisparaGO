package message

import "time"

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusDelivered  Status = "delivered"
	StatusRead       Status = "read"
	StatusFailed     Status = "failed"
)

type Message struct {
	ID                string     `json:"id"`
	CampaignID        string     `json:"campaign_id"`
	RecipientPhone    string     `json:"recipient_phone"`
	Content           string     `json:"content"`
	Status            Status     `json:"status"`
	ProviderMessageID string     `json:"provider_message_id"`
	AttemptCount      int        `json:"attempt_count"`
	LastError         string     `json:"last_error"`
	NextRetryAt       *time.Time `json:"next_retry_at"`
	SentAt            *time.Time `json:"sent_at"`
	DeliveredAt       *time.Time `json:"delivered_at"`
	ReadAt            *time.Time `json:"read_at"`
	FailedAt          *time.Time `json:"failed_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
