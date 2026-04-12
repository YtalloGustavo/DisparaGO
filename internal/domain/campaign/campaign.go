package campaign

import "time"

type Status string

const (
	StatusDraft      Status = "draft"
	StatusScheduled  Status = "scheduled"
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
	ID               string     `json:"id"`
	CompanyID        int64      `json:"company_id"`
	CreatedByUserID  *int64     `json:"created_by_user_id,omitempty"`
	Name             string     `json:"name"`
	InstanceID       string     `json:"instance_id"`
	Message          string     `json:"message"`
	Status           Status     `json:"status"`
	SendMode         string     `json:"send_mode"`
	ScheduledAtUTC   *time.Time `json:"scheduled_at_utc,omitempty"`
	ScheduledAt      *time.Time `json:"scheduled_at,omitempty"`
	ScheduledTZ      string     `json:"scheduled_timezone,omitempty"`
	ReleasedAt       *time.Time `json:"released_at,omitempty"`
	CancelledAt      *time.Time `json:"cancelled_at,omitempty"`
	ExternalSource   string     `json:"external_source,omitempty"`
	ExternalSourceID string     `json:"external_source_id,omitempty"`
	TotalCount       int        `json:"total_messages"`
	PendingCount     int        `json:"pending_count"`
	ProcessingCount  int        `json:"processing_count"`
	SentCount        int        `json:"sent_count"`
	DeliveredCount   int        `json:"delivered_count"`
	ReadCount        int        `json:"read_count"`
	FailedCount      int        `json:"failed_count"`
	Paused           bool       `json:"paused"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
