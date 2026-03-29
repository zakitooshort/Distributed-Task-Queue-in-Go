package store

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/ouzai/task-queue/internal/queue"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// JobRecord is the gorm model for the jobs table
type JobRecord struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey"`
	Queue       string     `gorm:"index;not null"`
	Type        string     `gorm:"index;not null"`
	Payload     []byte     `gorm:"type:jsonb"`
	Status      string     `gorm:"index;not null;default:pending"`
	Priority    int        `gorm:"default:0"`
	Attempts    int        `gorm:"default:0"`
	MaxAttempts int        `gorm:"default:3"`
	ScheduledAt *time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	FailedAt    *time.Time
	Error       string
	CreatedAt   time.Time
	WorkerID    string
}

func (JobRecord) TableName() string { return "jobs" }

// WorkerRecord is the gorm model for the workers table
type WorkerRecord struct {
	ID         string     `gorm:"primaryKey"             json:"id"`
	Status     string     `gorm:"not null;default:active" json:"status"`
	Queues     string     `json:"queues"`
	CurrentJob *uuid.UUID `gorm:"type:uuid"              json:"current_job"`
	LastSeen   time.Time  `json:"last_seen"`
	StartedAt  time.Time  `json:"started_at"`
}

func (WorkerRecord) TableName() string { return "workers" }

// PostgresStore wraps the gorm db
type PostgresStore struct {
	db *gorm.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		host, port, user, pass, name,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	// run migrations
	if err := db.AutoMigrate(&JobRecord{}, &WorkerRecord{}); err != nil {
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) CreateJob(j *queue.Job) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	j.CreatedAt = time.Now().UTC()

	rec := jobToRecord(j)
	return s.db.Create(rec).Error
}

func (s *PostgresStore) GetJob(id uuid.UUID) (*queue.Job, error) {
	var rec JobRecord
	if err := s.db.First(&rec, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return recordToJob(&rec), nil
}

// UpdateJobStatus updates status and any related timestamps
func (s *PostgresStore) UpdateJobStatus(id uuid.UUID, status queue.Status, updates map[string]any) error {
	updates["status"] = string(status)
	return s.db.Model(&JobRecord{}).Where("id = ?", id).Updates(updates).Error
}

type ListJobsFilter struct {
	Status string
	Queue  string
	Type   string
	Page   int
	Limit  int
}

func (s *PostgresStore) ListJobs(f ListJobsFilter) ([]queue.Job, int64, error) {
	q := s.db.Model(&JobRecord{})

	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.Queue != "" {
		q = q.Where("queue = ?", f.Queue)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}

	var total int64
	q.Count(&total)

	if f.Limit == 0 {
		f.Limit = 50
	}
	if f.Page < 1 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit

	var records []JobRecord
	if err := q.Order("created_at desc").Limit(f.Limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	jobs := make([]queue.Job, len(records))
	for i, r := range records {
		jobs[i] = *recordToJob(&r)
	}
	return jobs, total, nil
}

// GetStuckJobs returns jobs that have been in "running" status for longer than the given timeout
func (s *PostgresStore) GetStuckJobs(timeout time.Duration) ([]queue.Job, error) {
	cutoff := time.Now().UTC().Add(-timeout)
	var records []JobRecord
	err := s.db.Where("status = ? AND started_at < ?", "running", cutoff).Find(&records).Error
	if err != nil {
		return nil, err
	}

	jobs := make([]queue.Job, len(records))
	for i, r := range records {
		jobs[i] = *recordToJob(&r)
	}
	return jobs, nil
}

func (s *PostgresStore) UpsertWorker(w *WorkerRecord) error {
	return s.db.Save(w).Error
}

func (s *PostgresStore) UpdateWorkerHeartbeat(id string, currentJob *uuid.UUID) error {
	updates := map[string]any{
		"last_seen":   time.Now().UTC(),
		"current_job": currentJob,
	}
	return s.db.Model(&WorkerRecord{}).Where("id = ?", id).Updates(updates).Error
}

func (s *PostgresStore) MarkWorkerStopped(id string) error {
	return s.db.Model(&WorkerRecord{}).Where("id = ?", id).Update("status", "stopped").Error
}

func (s *PostgresStore) ListWorkers() ([]WorkerRecord, error) {
	var workers []WorkerRecord
	// only show workers seen in the last 2 minutes as "active"
	cutoff := time.Now().UTC().Add(-2 * time.Minute)
	err := s.db.Where("last_seen > ?", cutoff).Find(&workers).Error
	return workers, err
}

// GetRecentlyUpdatedJobs returns jobs whose status changed after the given time
func (s *PostgresStore) GetRecentlyUpdatedJobs(since time.Time) ([]queue.Job, error) {
	var records []JobRecord
	err := s.db.Where(
		"started_at > ? OR completed_at > ? OR failed_at > ?",
		since, since, since,
	).Find(&records).Error
	if err != nil {
		return nil, err
	}
	jobs := make([]queue.Job, len(records))
	for i, r := range records {
		jobs[i] = *recordToJob(&r)
	}
	return jobs, nil
}

// GetThroughput returns how many jobs completed in the last intervalSeconds
func (s *PostgresStore) GetThroughput(intervalSeconds int) (int64, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(intervalSeconds) * time.Second)
	var count int64
	err := s.db.Model(&JobRecord{}).
		Where("status = ? AND completed_at > ?", "completed", cutoff).
		Count(&count).Error
	return count, err
}

// helpers

func jobToRecord(j *queue.Job) *JobRecord {
	return &JobRecord{
		ID:          j.ID,
		Queue:       j.Queue,
		Type:        j.Type,
		Payload:     []byte(j.Payload),
		Status:      string(j.Status),
		Priority:    j.Priority,
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		ScheduledAt: j.ScheduledAt,
		StartedAt:   j.StartedAt,
		CompletedAt: j.CompletedAt,
		FailedAt:    j.FailedAt,
		Error:       j.Error,
		CreatedAt:   j.CreatedAt,
		WorkerID:    j.WorkerID,
	}
}

func recordToJob(r *JobRecord) *queue.Job {
	return &queue.Job{
		ID:          r.ID,
		Queue:       r.Queue,
		Type:        r.Type,
		Payload:     r.Payload,
		Status:      queue.Status(r.Status),
		Priority:    r.Priority,
		Attempts:    r.Attempts,
		MaxAttempts: r.MaxAttempts,
		ScheduledAt: r.ScheduledAt,
		StartedAt:   r.StartedAt,
		CompletedAt: r.CompletedAt,
		FailedAt:    r.FailedAt,
		Error:       r.Error,
		CreatedAt:   r.CreatedAt,
		WorkerID:    r.WorkerID,
	}
}
