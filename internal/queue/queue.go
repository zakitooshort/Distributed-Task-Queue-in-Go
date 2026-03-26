package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// these interfaces let us keep the queue logic decoupled from the concrete store implementations

type DBStore interface {
	CreateJob(j *Job) error
	GetJob(id uuid.UUID) (*Job, error)
	UpdateJobStatus(id uuid.UUID, status Status, updates map[string]any) error
}

type RedisStore interface {
	Enqueue(ctx context.Context, queueName string, jobID uuid.UUID) error
	EnqueuePriority(ctx context.Context, queueName string, jobID uuid.UUID, priority int) error
	EnqueueDelayed(ctx context.Context, jobID uuid.UUID, runAt time.Time) error
	EnqueueDead(ctx context.Context, jobID uuid.UUID, jobJSON []byte) error
	RemoveFromDead(ctx context.Context, jobID uuid.UUID) error
	QueueDepth(ctx context.Context, queueName string) (int64, error)
}

// Queue is the main entry point for enqueuing and managing jobs
type Queue struct {
	db    DBStore
	redis RedisStore
}

func New(db DBStore, redis RedisStore) *Queue {
	return &Queue{db: db, redis: redis}
}

// Enqueue creates a job and pushes it to redis immediately
func (q *Queue) Enqueue(ctx context.Context, req *EnqueueRequest) (*Job, error) {
	if req.Queue == "" {
		req.Queue = QueueDefault
	}
	if req.MaxAttempts == 0 {
		req.MaxAttempts = 3
	}
	if req.Payload == nil {
		req.Payload = json.RawMessage("{}")
	}

	job := &Job{
		ID:          uuid.New(),
		Queue:       req.Queue,
		Type:        req.Type,
		Payload:     req.Payload,
		Status:      StatusPending,
		Priority:    req.Priority,
		MaxAttempts: req.MaxAttempts,
		CreatedAt:   time.Now().UTC(),
	}

	// handle delayed jobs
	if req.DelaySeconds > 0 {
		runAt := time.Now().UTC().Add(time.Duration(req.DelaySeconds) * time.Second)
		job.ScheduledAt = &runAt

		if err := q.db.CreateJob(job); err != nil {
			return nil, fmt.Errorf("saving delayed job to postgres: %w", err)
		}
		if err := q.redis.EnqueueDelayed(ctx, job.ID, runAt); err != nil {
			return nil, fmt.Errorf("adding to delayed set: %w", err)
		}
		return job, nil
	}

	// save to postgres first
	if err := q.db.CreateJob(job); err != nil {
		return nil, fmt.Errorf("saving job to postgres: %w", err)
	}

	// then push to redis
	if err := q.pushToRedis(ctx, job); err != nil {
		// job is in postgres as pending, scheduler will eventually pick it up
		// so this isn't fatal — just log it
		return job, fmt.Errorf("pushing to redis (job saved to postgres, will be recovered): %w", err)
	}

	return job, nil
}

// ScheduleRetry re-enqueues a failed job with exponential backoff delay
func (q *Queue) ScheduleRetry(ctx context.Context, job *Job, errMsg string) error {
	now := time.Now().UTC()

	updates := map[string]any{
		"attempts":  job.Attempts + 1,
		"failed_at": now,
		"error":     errMsg,
	}

	if job.Attempts+1 >= job.MaxAttempts {
		// exhausted retries — move to dead letter
		return q.MoveToDeadLetter(ctx, job, errMsg)
	}

	delay := RetryDelay(job.Attempts)
	runAt := now.Add(delay)
	updates["status"] = string(StatusPending)
	updates["scheduled_at"] = runAt

	if err := q.db.UpdateJobStatus(job.ID, StatusPending, updates); err != nil {
		return fmt.Errorf("updating job for retry: %w", err)
	}

	return q.redis.EnqueueDelayed(ctx, job.ID, runAt)
}

// MoveToDeadLetter marks the job as dead in postgres and pushes to the dead redis list
func (q *Queue) MoveToDeadLetter(ctx context.Context, job *Job, errMsg string) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"attempts":  job.Attempts + 1,
		"failed_at": now,
		"error":     errMsg,
	}

	if err := q.db.UpdateJobStatus(job.ID, StatusDead, updates); err != nil {
		return fmt.Errorf("marking job as dead: %w", err)
	}

	// push a snapshot to the redis dead list for quick inspection
	job.Status = StatusDead
	job.Error = errMsg
	job.Attempts = job.Attempts + 1
	job.FailedAt = &now

	b, _ := json.Marshal(job)
	return q.redis.EnqueueDead(ctx, job.ID, b)
}

// ReplayDeadJob re-enqueues a dead job so it gets another shot
func (q *Queue) ReplayDeadJob(ctx context.Context, id uuid.UUID) (*Job, error) {
	job, err := q.db.GetJob(id)
	if err != nil {
		return nil, fmt.Errorf("job not found: %w", err)
	}
	if job.Status != StatusDead {
		return nil, fmt.Errorf("job is not in dead status (it's %s)", job.Status)
	}

	updates := map[string]any{
		"status":   string(StatusPending),
		"attempts": 0,
		"error":    "",
	}
	if err := q.db.UpdateJobStatus(id, StatusPending, updates); err != nil {
		return nil, fmt.Errorf("updating job status: %w", err)
	}

	// remove from dead letter redis list
	q.redis.RemoveFromDead(ctx, id)

	// re-enqueue
	job.Status = StatusPending
	job.Attempts = 0
	if err := q.pushToRedis(ctx, job); err != nil {
		return nil, fmt.Errorf("re-enqueuing job: %w", err)
	}

	return job, nil
}

func (q *Queue) pushToRedis(ctx context.Context, job *Job) error {
	if job.Priority > 0 {
		return q.redis.EnqueuePriority(ctx, job.Queue, job.ID, job.Priority)
	}
	return q.redis.Enqueue(ctx, job.Queue, job.ID)
}
