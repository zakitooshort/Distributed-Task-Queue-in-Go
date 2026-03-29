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

	"math/rand"

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
	// process_payment — goes to critical queue, fails 30% of the time to show retry/DLQ flow
	w.RegisterHandler("process_payment", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			OrderID  string  `json:"order_id"`
			Customer string  `json:"customer"`
			Amount   float64 `json:"amount"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}
		slog.Info("processing payment", "order_id", p.OrderID, "customer", p.Customer, "amount", p.Amount)
		time.Sleep(10 * time.Second)
		if rand.Float32() < 0.3 {
			return fmt.Errorf("payment declined for order %s (simulated)", p.OrderID)
		}
		slog.Info("payment accepted", "order_id", p.OrderID, "amount", p.Amount)
		return nil
	})

	// send_confirmation_email — goes to email queue, always succeeds
	w.RegisterHandler("send_confirmation_email", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			OrderID       string `json:"order_id"`
			Customer      string `json:"customer"`
			CustomerEmail string `json:"customer_email"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}
		slog.Info("sending confirmation email", "order_id", p.OrderID, "to", p.CustomerEmail)
		time.Sleep(10 * time.Second)
		slog.Info("confirmation email sent", "order_id", p.OrderID, "to", p.CustomerEmail)
		return nil
	})

	// update_inventory — goes to default queue, always succeeds
	w.RegisterHandler("update_inventory", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			OrderID string   `json:"order_id"`
			Items   []string `json:"items"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}
		slog.Info("updating inventory", "order_id", p.OrderID, "items", p.Items)
		time.Sleep(10 * time.Second)
		slog.Info("inventory updated", "order_id", p.OrderID)
		return nil
	})

	// generate_invoice — goes to default queue, always succeeds
	w.RegisterHandler("generate_invoice", func(ctx context.Context, payload json.RawMessage) error {
		var p struct {
			OrderID  string  `json:"order_id"`
			Customer string  `json:"customer"`
			Total    float64 `json:"total"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return fmt.Errorf("invalid payload: %w", err)
		}
		slog.Info("generating invoice", "order_id", p.OrderID, "customer", p.Customer, "total", p.Total)
		time.Sleep(10 * time.Second)
		slog.Info("invoice generated", "order_id", p.OrderID)
		return nil
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
