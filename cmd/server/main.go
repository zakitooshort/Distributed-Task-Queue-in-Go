package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/ouzai/task-queue/internal/api"
	"github.com/ouzai/task-queue/internal/queue"
	"github.com/ouzai/task-queue/internal/scheduler"
	"github.com/ouzai/task-queue/internal/store"
)

func main() {
	godotenv.Load()
	setupLogging()

	slog.Info("starting server")

	// connect to postgres — this also runs AutoMigrate
	db, err := store.NewPostgresStore()
	if err != nil {
		slog.Error("postgres connection failed", "err", err)
		os.Exit(1)
	}

	// connect to redis
	redis, err := store.NewRedisStore()
	if err != nil {
		slog.Error("redis connection failed", "err", err)
		os.Exit(1)
	}

	q := queue.New(db, redis)
	broadcaster := api.NewBroadcaster()
	handler := api.NewHandler(db, redis, q, broadcaster)

	// set up gin
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(gin.Logger())

	handler.RegisterRoutes(r)

	// serve the built dashboard if it exists
	r.Static("/app", "./dashboard/dist")
	r.NoRoute(func(c *gin.Context) {
		// for SPA routing — serve index.html for unmatched routes
		if _, err := os.Stat("./dashboard/dist/index.html"); err == nil {
			c.File("./dashboard/dist/index.html")
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	// start the scheduler in background
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	sched := scheduler.New(db, redis)
	go sched.Start(ctx)

	// broadcast queue stats every 5 seconds
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				handler.BroadcastQueueStats(ctx)
			}
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// start server in background so we can shut it down gracefully
	go func() {
		slog.Info("server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "err", err)
	}

	slog.Info("server stopped")
}

// corsMiddleware allows requests from the dashboard running on a different port during dev
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func setupLogging() {
	h := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(h))
}
