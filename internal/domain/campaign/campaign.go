package campaign

import "time"

type Status string

const (
	StatusDraft      Status = "draft"
	StatusPending    Status = "pending"
	StatusPaused     Status = "paused"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusDelivered  Status = "delivered"
	StatusRead       Status = "read"
	StatusFailed     Status = "failed"
	StatusPartial    Status = "partial"
)

type Campaign struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	InstanceID      string    `json:"instance_id"`
	Message         string    `json:"message"`
	Status          Status    `json:"status"`
	TotalCount      int       `json:"total_messages"`
	PendingCount    int       `json:"pending_count"`
	ProcessingCount int       `json:"processing_count"`
	SentCount       int       `json:"sent_count"`
	DeliveredCount  int       `json:"delivered_count"`
	ReadCount       int       `json:"read_count"`
	FailedCount     int       `json:"failed_count"`
	Paused          bool      `json:"paused"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
