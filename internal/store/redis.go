package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// main queues
	redisQueuePrefix = "queue:"
	// sorted set for delayed jobs — score is the unix timestamp to run at
	delayedKey = "queue:delayed"
	// dead letter queue
	deadKey = "queue:dead"
	// sorted set for priority queues — score is priority (higher = first)
	priorityQueuePrefix = "queue:priority:"
)

// RedisStore wraps the go-redis client
type RedisStore struct {
	client *redis.Client
}

func NewRedisStore() (*RedisStore, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return &RedisStore{client: client}, nil
}

// Enqueue pushes a job id to the left of the queue list (LPUSH)
// workers do BRPOP from the right, so this is FIFO
func (r *RedisStore) Enqueue(ctx context.Context, queueName string, jobID uuid.UUID) error {
	key := redisQueuePrefix + queueName
	return r.client.LPush(ctx, key, jobID.String()).Err()
}

// EnqueuePriority adds a job to the priority sorted set
// score = priority, higher priority = popped first (we use BZPOPMAX)
func (r *RedisStore) EnqueuePriority(ctx context.Context, queueName string, jobID uuid.UUID, priority int) error {
	key := priorityQueuePrefix + queueName
	return r.client.ZAdd(ctx, key, redis.Z{
		Score:  float64(priority),
		Member: jobID.String(),
	}).Err()
}

// DequeueResult holds what came back from a BRPOP
type DequeueResult struct {
	Queue string
	JobID uuid.UUID
}

// Dequeue blocks until a job is available in any of the given queues
// tries priority queue first, then falls back to the regular list
func (r *RedisStore) Dequeue(ctx context.Context, queues []string, timeout time.Duration) (*DequeueResult, error) {
	// check priority queues first (non-blocking)
	for _, q := range queues {
		priorityKey := priorityQueuePrefix + q
		result, err := r.client.ZPopMax(ctx, priorityKey, 1).Result()
		if err == nil && len(result) > 0 {
			id, err := uuid.Parse(result[0].Member.(string))
			if err != nil {
				continue
			}
			return &DequeueResult{Queue: q, JobID: id}, nil
		}
	}

	// fall back to blocking pop on the regular queues
	keys := make([]string, len(queues))
	for i, q := range queues {
		keys[i] = redisQueuePrefix + q
	}

	result, err := r.client.BRPop(ctx, timeout, keys...).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // timeout, nothing arrived
		}
		return nil, fmt.Errorf("brpop: %w", err)
	}

	// result[0] = key name, result[1] = value
	queueName := result[0][len(redisQueuePrefix):]
	id, err := uuid.Parse(result[1])
	if err != nil {
		return nil, fmt.Errorf("parsing job id from redis: %w", err)
	}

	return &DequeueResult{Queue: queueName, JobID: id}, nil
}

// EnqueueDelayed adds a job to the delayed sorted set with a future unix timestamp as the score
func (r *RedisStore) EnqueueDelayed(ctx context.Context, jobID uuid.UUID, runAt time.Time) error {
	return r.client.ZAdd(ctx, delayedKey, redis.Z{
		Score:  float64(runAt.Unix()),
		Member: jobID.String(),
	}).Err()
}

// GetDueDelayed pops all delayed jobs whose score (run time) is <= now
// returns their IDs so the scheduler can push them to the main queue
func (r *RedisStore) GetDueDelayed(ctx context.Context) ([]uuid.UUID, error) {
	now := float64(time.Now().Unix())
	members, err := r.client.ZRangeByScoreWithScores(ctx, delayedKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(members))
	memberStrs := make([]any, len(members))
	for i, m := range members {
		id, err := uuid.Parse(m.Member.(string))
		if err != nil {
			continue
		}
		ids = append(ids, id)
		memberStrs[i] = m.Member
	}

	// remove them atomically
	r.client.ZRem(ctx, delayedKey, memberStrs...)

	return ids, nil
}

// EnqueueDead pushes a job snapshot to the dead letter list
func (r *RedisStore) EnqueueDead(ctx context.Context, jobID uuid.UUID, jobJSON []byte) error {
	// store the full job json in the dead list so we can inspect it without hitting postgres
	entry := map[string]any{
		"id":         jobID.String(),
		"data":       string(jobJSON),
		"dead_at":    time.Now().UTC().Format(time.RFC3339),
	}
	b, _ := json.Marshal(entry)
	return r.client.LPush(ctx, deadKey, string(b)).Err()
}

// ListDead returns the last N entries from the dead letter queue
func (r *RedisStore) ListDead(ctx context.Context, limit int64) ([]string, error) {
	return r.client.LRange(ctx, deadKey, 0, limit-1).Result()
}

// RemoveFromDead removes a specific job from the dead letter list by id
func (r *RedisStore) RemoveFromDead(ctx context.Context, jobID uuid.UUID) error {
	// get all entries and remove the matching one
	entries, err := r.client.LRange(ctx, deadKey, 0, -1).Result()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		var m map[string]any
		if err := json.Unmarshal([]byte(entry), &m); err != nil {
			continue
		}
		if m["id"] == jobID.String() {
			r.client.LRem(ctx, deadKey, 1, entry)
			return nil
		}
	}
	return nil
}

// QueueDepth returns how many jobs are waiting in a queue
func (r *RedisStore) QueueDepth(ctx context.Context, queueName string) (int64, error) {
	regular, err := r.client.LLen(ctx, redisQueuePrefix+queueName).Result()
	if err != nil {
		return 0, err
	}
	priority, err := r.client.ZCard(ctx, priorityQueuePrefix+queueName).Result()
	if err != nil {
		return regular, nil // not fatal
	}
	return regular + priority, nil
}

// DelayedCount returns how many jobs are waiting in the delayed set
func (r *RedisStore) DelayedCount(ctx context.Context) (int64, error) {
	return r.client.ZCard(ctx, delayedKey).Result()
}

// DeadCount returns how many jobs are in the dead letter queue
func (r *RedisStore) DeadCount(ctx context.Context) (int64, error) {
	return r.client.LLen(ctx, deadKey).Result()
}
