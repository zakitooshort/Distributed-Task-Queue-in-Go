package queue

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusDead      Status = "dead"
)

// the queues we support by default
const (
	QueueDefault  = "default"
	QueueCritical = "critical"
	QueueEmail    = "email"
)

// Job is what gets pushed into the queue and stored in postgres
type Job struct {
	ID          uuid.UUID       `json:"id"`
	Queue       string          `json:"queue"`
	Type        string          `json:"type"`
	Payload     json.RawMessage `json:"payload"`
	Status      Status          `json:"status"`
	Priority    int             `json:"priority"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	FailedAt    *time.Time      `json:"failed_at,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	WorkerID    string          `json:"worker_id,omitempty"`
}

// EnqueueRequest is what the API accepts from the client
type EnqueueRequest struct {
	Queue       string          `json:"queue"`
	Type        string          `json:"type" binding:"required"`
	Payload     json.RawMessage `json:"payload"`
	Priority    int             `json:"priority"`
	MaxAttempts int             `json:"max_attempts"`
	// delay in seconds, 0 means run immediately
	DelaySeconds int `json:"delay_seconds"`
}

// RetryDelay calculates how long to wait before the next attempt
// formula: 10s * 10^attempt
func RetryDelay(attempt int) time.Duration {
	base := 10.0
	factor := 10.0
	delay := base
	for i := 0; i < attempt; i++ {
		delay *= factor
	}
	// cap at ~28 hours so it doesn't get insane
	max := 100_000.0
	if delay > max {
		delay = max
	}
	return time.Duration(delay) * time.Second
}
