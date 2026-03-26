package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ouzai/task-queue/internal/queue"
	"github.com/ouzai/task-queue/internal/store"
)

// Handler holds all the dependencies the route handlers need
type Handler struct {
	db          *store.PostgresStore
	redis       *store.RedisStore
	q           *queue.Queue
	broadcaster *Broadcaster
}

func NewHandler(db *store.PostgresStore, redis *store.RedisStore, q *queue.Queue, broadcaster *Broadcaster) *Handler {
	return &Handler{
		db:          db,
		redis:       redis,
		q:           q,
		broadcaster: broadcaster,
	}
}

// RegisterRoutes wires up all the API routes on the gin engine
func (h *Handler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.POST("/jobs", h.CreateJob)
		api.GET("/jobs", h.ListJobs)
		api.GET("/jobs/:id", h.GetJob)
		api.POST("/jobs/:id/retry", h.RetryJob)
		api.DELETE("/jobs/:id", h.CancelJob)
		api.GET("/queues", h.GetQueueStats)
		api.GET("/workers", h.GetWorkers)
	}

	// SSE endpoint — not under /api because it doesn't return JSON
	r.GET("/events", h.StreamEvents)
}

// POST /api/jobs
func (h *Handler) CreateJob(c *gin.Context) {
	var req queue.EnqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.q.Enqueue(c.Request.Context(), &req)
	if err != nil {
		slog.Error("failed to enqueue job", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enqueue job"})
		return
	}

	h.broadcaster.Broadcast("job.created", job)
	c.JSON(http.StatusCreated, job)
}

// GET /api/jobs
func (h *Handler) ListJobs(c *gin.Context) {
	filter := store.ListJobsFilter{
		Status: c.Query("status"),
		Queue:  c.Query("queue"),
		Type:   c.Query("type"),
	}

	if p := c.Query("page"); p != "" {
		filter.Page, _ = strconv.Atoi(p)
	}
	if l := c.Query("limit"); l != "" {
		filter.Limit, _ = strconv.Atoi(l)
	}

	jobs, total, err := h.db.ListJobs(filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list jobs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":  jobs,
		"total": total,
		"page":  filter.Page,
		"limit": filter.Limit,
	})
}

// GET /api/jobs/:id
func (h *Handler) GetJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	job, err := h.db.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	c.JSON(http.StatusOK, job)
}

// POST /api/jobs/:id/retry — replay a dead job
func (h *Handler) RetryJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	job, err := h.q.ReplayDeadJob(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.broadcaster.Broadcast("job.created", job)
	c.JSON(http.StatusOK, job)
}

// DELETE /api/jobs/:id — cancel a pending job
func (h *Handler) CancelJob(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	job, err := h.db.GetJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}

	if job.Status != queue.StatusPending {
		c.JSON(http.StatusBadRequest, gin.H{"error": "can only cancel pending jobs"})
		return
	}

	now := time.Now().UTC()
	if err := h.db.UpdateJobStatus(id, queue.StatusFailed, map[string]any{
		"failed_at": now,
		"error":     "cancelled by user",
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel job"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "job cancelled"})
}

// GET /api/queues — queue depths + throughput stats
func (h *Handler) GetQueueStats(c *gin.Context) {
	ctx := c.Request.Context()

	queues := []string{"default", "critical", "email"}
	stats := make([]gin.H, 0, len(queues))

	for _, q := range queues {
		depth, _ := h.redis.QueueDepth(ctx, q)
		stats = append(stats, gin.H{
			"queue": q,
			"depth": depth,
		})
	}

	delayed, _ := h.redis.DelayedCount(ctx)
	dead, _ := h.redis.DeadCount(ctx)
	throughput, _ := h.db.GetThroughput(60)

	c.JSON(http.StatusOK, gin.H{
		"queues":     stats,
		"delayed":    delayed,
		"dead":       dead,
		"throughput": throughput,
	})
}

// GET /api/workers
func (h *Handler) GetWorkers(c *gin.Context) {
	workers, err := h.db.ListWorkers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list workers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"workers": workers})
}

// GET /events — SSE stream for the dashboard
func (h *Handler) StreamEvents(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // tells nginx not to buffer SSE

	ch, cleanup := h.broadcaster.Register()
	defer cleanup()

	// send connected ping right away so the client knows it's alive
	fmt.Fprint(c.Writer, "data: {\"type\":\"connected\"}\n\n")
	c.Writer.Flush()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			msg, err := FormatSSE(event)
			if err != nil {
				slog.Error("failed to format sse event", "err", err)
				continue
			}
			fmt.Fprint(c.Writer, msg)
			c.Writer.Flush()
		}
	}
}

// BroadcastQueueStats pushes live queue stats to all connected SSE clients
// call this on a ticker (e.g. every 5s) from the server main loop
func (h *Handler) BroadcastQueueStats(ctx context.Context) {
	queues := []string{"default", "critical", "email"}
	stats := make([]map[string]any, 0, len(queues))

	for _, q := range queues {
		depth, _ := h.redis.QueueDepth(ctx, q)
		stats = append(stats, map[string]any{
			"queue": q,
			"depth": depth,
		})
	}

	delayed, _ := h.redis.DelayedCount(ctx)
	dead, _ := h.redis.DeadCount(ctx)
	throughput, _ := h.db.GetThroughput(60)

	h.broadcaster.Broadcast("queue.stats", map[string]any{
		"queues":     stats,
		"delayed":    delayed,
		"dead":       dead,
		"throughput": throughput,
		"timestamp":  time.Now().UTC(),
	})
}
