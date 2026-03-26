package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ouzai/task-queue/internal/queue"
	"github.com/ouzai/task-queue/internal/store"
	"github.com/ouzai/task-queue/internal/worker"
)

func main() {
	// load .env if present
	godotenv.Load()

	setupLogging()

	// figure out which queues and concurrency from env
	queuesStr := getEnv("WORKER_QUEUES", "default")
	queues := strings.Split(queuesStr, ",")
	for i := range queues {
		queues[i] = strings.TrimSpace(queues[i])
	}

	concurrency := getEnvInt("WORKER_CONCURRENCY", 5)

	workerID := buildWorkerID()

	slog.Info("starting worker", "id", workerID, "queues", queues, "concurrency", concurrency)

	// init stores
	db, err := store.NewPostgresStore()
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}

	redis, err := store.NewRedisStore()
	if err != nil {
		slog.Error("failed to connect to redis", "err", err)
		os.Exit(1)
	}

	q := queue.New(db, redis)

	cfg := worker.Config{
		ID:          workerID,
		Queues:      queues,
		Concurrency: concurrency,
	}

	w := worker.New(cfg, db, redis, q, nil) // nil broadcaster — workers don't broadcast SSE directly

	// register your job handlers here
	registerHandlers(w)

	// graceful shutdown on SIGINT / SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := w.Start(ctx); err != nil {
		slog.Error("worker error", "err", err)
		os.Exit(1)
	}
}

// registerHandlers is where you add your actual job type implementations
func registerHandlers(w *worker.Worker) {
	// example: send email
	w.RegisterHandler("send_email", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			To      string `json:"to"`
			Subject string `json:"subject"`
			Body    string `json:"body"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		// simulate some work
		slog.Info("sending email", "to", p.To, "subject", p.Subject)
		time.Sleep(500 * time.Millisecond)
		slog.Info("email sent", "to", p.To)
		return nil
	})

	// example: process image
	w.RegisterHandler("process_image", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			ImageURL string `json:"image_url"`
			Format   string `json:"format"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}

		slog.Info("processing image", "url", p.ImageURL, "format", p.Format)
		time.Sleep(2 * time.Second)
		slog.Info("image processed", "url", p.ImageURL)
		return nil
	})

	// a job type that always fails — useful for testing the retry/DLQ flow
	w.RegisterHandler("failing_job", func(ctx context.Context, payload json.RawMessage) error {
		return fmt.Errorf("this job always fails (for testing)")
	})
}

func buildWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return fallback
}

func setupLogging() {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(h))
}
