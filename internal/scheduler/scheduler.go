package scheduler

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ouzai/task-queue/internal/queue"
	"github.com/ouzai/task-queue/internal/store"
)

// these interfaces keep the scheduler decoupled from the concrete store types

type redisStore interface {
	GetDueDelayed(ctx context.Context) ([]uuid.UUID, error)
	Enqueue(ctx context.Context, queueName string, jobID uuid.UUID) error
	EnqueuePriority(ctx context.Context, queueName string, jobID uuid.UUID, priority int) error
}

type dbStore interface {
	GetJob(id uuid.UUID) (*queue.Job, error)
	GetStuckJobs(timeout time.Duration) ([]queue.Job, error)
	UpdateJobStatus(id uuid.UUID, status queue.Status, updates map[string]any) error
}

// Scheduler handles two things:
// 1. promoting delayed jobs when their time comes (every 1s)
// 2. recovering stuck jobs that are in "running" state for too long (every 60s)
type Scheduler struct {
	db    dbStore
	redis redisStore
}

func New(db *store.PostgresStore, redis *store.RedisStore) *Scheduler {
	return &Scheduler{db: db, redis: redis}
}

// Start runs the scheduler loops — call this in a goroutine
func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler started")

	delayedTicker := time.NewTicker(time.Second)
	stuckTicker := time.NewTicker(60 * time.Second)
	defer delayedTicker.Stop()
	defer stuckTicker.Stop()

	// run stuck job check once on startup too
	go s.recoverStuckJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler stopping")
			return
		case <-delayedTicker.C:
			s.promoteDelayedJobs(ctx)
		case <-stuckTicker.C:
			s.recoverStuckJobs(ctx)
		}
	}
}

// promoteDelayedJobs checks the delayed sorted set and moves any due jobs to their queues
func (s *Scheduler) promoteDelayedJobs(ctx context.Context) {
	ids, err := s.redis.GetDueDelayed(ctx)
	if err != nil {
		slog.Error("error getting due delayed jobs", "err", err)
		return
	}

	for _, id := range ids {
		job, err := s.db.GetJob(id)
		if err != nil {
			slog.Error("couldn't load delayed job", "id", id, "err", err)
			continue
		}

		// reset status to pending before re-queuing
		if err := s.db.UpdateJobStatus(job.ID, queue.StatusPending, map[string]any{
			"status": string(queue.StatusPending),
		}); err != nil {
			slog.Error("couldn't reset delayed job status", "id", id, "err", err)
			continue
		}

		if err := s.enqueueJob(ctx, job); err != nil {
			slog.Error("failed to promote delayed job", "id", id, "err", err)
			continue
		}

		slog.Info("promoted delayed job", "id", id, "type", job.Type, "queue", job.Queue)
	}
}

// recoverStuckJobs finds jobs that have been running for too long and re-enqueues them
func (s *Scheduler) recoverStuckJobs(ctx context.Context) {
	timeout := getStuckTimeout()

	jobs, err := s.db.GetStuckJobs(timeout)
	if err != nil {
		slog.Error("error finding stuck jobs", "err", err)
		return
	}

	if len(jobs) == 0 {
		return
	}

	slog.Warn("found stuck jobs, recovering", "count", len(jobs))

	for _, job := range jobs {
		// reset to pending
		if err := s.db.UpdateJobStatus(job.ID, queue.StatusPending, map[string]any{
			"status":    string(queue.StatusPending),
			"worker_id": "",
		}); err != nil {
			slog.Error("couldn't reset stuck job", "id", job.ID, "err", err)
			continue
		}

		if err := s.enqueueJob(ctx, &job); err != nil {
			slog.Error("failed to re-enqueue stuck job", "id", job.ID, "err", err)
			continue
		}

		slog.Info("recovered stuck job", "id", job.ID, "type", job.Type, "queue", job.Queue)
	}
}

func (s *Scheduler) enqueueJob(ctx context.Context, job *queue.Job) error {
	if job.Priority > 0 {
		return s.redis.EnqueuePriority(ctx, job.Queue, job.ID, job.Priority)
	}
	return s.redis.Enqueue(ctx, job.Queue, job.ID)
}

func getStuckTimeout() time.Duration {
	minutes := 5
	if v := os.Getenv("STUCK_JOB_TIMEOUT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			minutes = n
		}
	}
	return time.Duration(minutes) * time.Minute
}
