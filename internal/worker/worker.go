package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ouzai/task-queue/internal/queue"
	"github.com/ouzai/task-queue/internal/store"
)

// HandlerFunc is the signature for job handlers
// each job type maps to one of these
type HandlerFunc func(ctx context.Context, payload json.RawMessage) error

// Broadcaster is the interface we need to send SSE events
// the actual implementation is in internal/api
type Broadcaster interface {
	Broadcast(event string, data any)
}

// Config holds everything the worker needs to start up
type Config struct {
	ID          string   // e.g. "hostname-12345"
	Queues      []string // which queues to listen on
	Concurrency int      // how many goroutines to run in parallel
}

// Worker is a single worker process
type Worker struct {
	cfg         Config
	db          *store.PostgresStore
	redis       *store.RedisStore
	q           *queue.Queue
	broadcaster Broadcaster
	handlers    map[string]HandlerFunc
	wg          sync.WaitGroup
}

func New(cfg Config, db *store.PostgresStore, redis *store.RedisStore, q *queue.Queue, broadcaster Broadcaster) *Worker {
	return &Worker{
		cfg:         cfg,
		db:          db,
		redis:       redis,
		q:           q,
		broadcaster: broadcaster,
		handlers:    make(map[string]HandlerFunc),
	}
}

// RegisterHandler adds a handler for a job type
func (w *Worker) RegisterHandler(jobType string, fn HandlerFunc) {
	w.handlers[jobType] = fn
}

// Start registers the worker in postgres and spawns the goroutine pool
func (w *Worker) Start(ctx context.Context) error {
	// register this worker in postgres
	workerRec := &store.WorkerRecord{
		ID:        w.cfg.ID,
		Status:    "active",
		Queues:    joinQueues(w.cfg.Queues),
		LastSeen:  time.Now().UTC(),
		StartedAt: time.Now().UTC(),
	}
	if err := w.db.UpsertWorker(workerRec); err != nil {
		return fmt.Errorf("registering worker: %w", err)
	}

	slog.Info("worker started", "id", w.cfg.ID, "queues", w.cfg.Queues, "concurrency", w.cfg.Concurrency)

	// start heartbeat
	go w.heartbeat(ctx)

	// spawn N worker goroutines
	for i := 0; i < w.cfg.Concurrency; i++ {
		w.wg.Add(1)
		go func(workerNum int) {
			defer w.wg.Done()
			w.loop(ctx, workerNum)
		}(i)
	}

	// block until context is cancelled
	<-ctx.Done()

	slog.Info("worker shutting down, waiting for in-flight jobs...", "id", w.cfg.ID)
	w.wg.Wait()

	// mark stopped in postgres
	if err := w.db.MarkWorkerStopped(w.cfg.ID); err != nil {
		slog.Error("failed to mark worker stopped", "err", err)
	}

	if w.broadcaster != nil {
		w.broadcaster.Broadcast("worker.stopped", map[string]string{"worker_id": w.cfg.ID})
	}

	slog.Info("worker stopped cleanly", "id", w.cfg.ID)
	return nil
}

// loop is the main dequeue → execute cycle for one goroutine
func (w *Worker) loop(ctx context.Context, num int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result, err := w.redis.Dequeue(ctx, w.cfg.Queues, 5*time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled, clean exit
			}
			slog.Error("dequeue error", "worker", num, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if result == nil {
			// timeout, nothing in the queue — loop again
			continue
		}

		w.processJob(ctx, result.JobID)
	}
}

func (w *Worker) processJob(ctx context.Context, jobID uuid.UUID) {
	job, err := w.db.GetJob(jobID)
	if err != nil {
		slog.Error("couldn't load job from postgres", "job_id", jobID, "err", err)
		return
	}

	// mark as running
	now := time.Now().UTC()
	err = w.db.UpdateJobStatus(job.ID, queue.StatusRunning, map[string]any{
		"started_at": now,
		"worker_id":  w.cfg.ID,
	})
	if err != nil {
		slog.Error("couldn't mark job as running", "job_id", jobID, "err", err)
		return
	}

	job.Status = queue.StatusRunning
	job.StartedAt = &now
	job.WorkerID = w.cfg.ID

	// update heartbeat with current job
	w.db.UpdateWorkerHeartbeat(w.cfg.ID, &job.ID)

	if w.broadcaster != nil {
		w.broadcaster.Broadcast("job.started", job)
	}

	slog.Info("running job", "job_id", job.ID, "type", job.Type, "attempt", job.Attempts+1)

	execErr := w.execute(ctx, job)

	// clear current job from heartbeat
	w.db.UpdateWorkerHeartbeat(w.cfg.ID, nil)

	if execErr != nil {
		slog.Warn("job failed", "job_id", job.ID, "type", job.Type, "err", execErr, "attempts", job.Attempts+1)

		retryErr := w.q.ScheduleRetry(ctx, job, execErr.Error())
		if retryErr != nil {
			slog.Error("failed to schedule retry", "job_id", job.ID, "err", retryErr)
		}

		if w.broadcaster != nil {
			if job.Attempts+1 >= job.MaxAttempts {
				w.broadcaster.Broadcast("job.dead", job)
			} else {
				w.broadcaster.Broadcast("job.failed", job)
			}
		}
		return
	}

	// success
	completedAt := time.Now().UTC()
	w.db.UpdateJobStatus(job.ID, queue.StatusCompleted, map[string]any{
		"completed_at": completedAt,
	})

	slog.Info("job completed", "job_id", job.ID, "type", job.Type)

	if w.broadcaster != nil {
		job.Status = queue.StatusCompleted
		job.CompletedAt = &completedAt
		w.broadcaster.Broadcast("job.completed", job)
	}
}

// execute runs the actual handler, recovering from panics
func (w *Worker) execute(ctx context.Context, job *queue.Job) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			err = fmt.Errorf("panic: %v\n%s", r, stack)
		}
	}()

	handler, ok := w.handlers[job.Type]
	if !ok {
		// no handler registered — treat as a simulated success so it doesn't retry forever
		// in production you'd probably want to dead-letter these immediately
		slog.Warn("no handler registered for job type, skipping", "type", job.Type)
		return nil
	}

	return handler(ctx, job.Payload)
}

// heartbeat updates the worker's last_seen timestamp every 30 seconds
func (w *Worker) heartbeat(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	hostname, _ := os.Hostname()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.db.UpdateWorkerHeartbeat(w.cfg.ID, nil); err != nil {
				slog.Error("heartbeat failed", "err", err)
			}
			if w.broadcaster != nil {
				w.broadcaster.Broadcast("worker.heartbeat", map[string]any{
					"worker_id": w.cfg.ID,
					"hostname":  hostname,
					"queues":    w.cfg.Queues,
					"timestamp": time.Now().UTC(),
				})
			}
		}
	}
}

func joinQueues(queues []string) string {
	result := ""
	for i, q := range queues {
		if i > 0 {
			result += ","
		}
		result += q
	}
	return result
}
