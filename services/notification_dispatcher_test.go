package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"split-udhar-apis/models"
	"split-udhar-apis/repositories"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupDispatcherTestDB(t *testing.T) *gorm.DB {
	dsn := fmt.Sprintf("file:memdb_dispatcher_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}

	if err := db.AutoMigrate(&models.NotificationOutbox{}, &models.User{}, &models.UserDevice{}); err != nil {
		t.Fatalf("failed to migrate schema: %v", err)
	}

	return db
}

func TestNotificationDispatcher_IdempotentEnqueue(t *testing.T) {
	db := setupDispatcherTestDB(t)
	fcmService := NewFCMService(db)
	dispatcher := NewNotificationDispatcher(db, fcmService)

	key := "tx_123_create"

	// 1. First enqueue
	job1, err := dispatcher.EnqueueNotification(key, "user", []string{"9876543210"}, "Test Title", "Test Body", map[string]string{"type": "tx"})
	if err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if job1.ID == 0 {
		t.Fatalf("expected valid job ID")
	}

	// 2. Duplicate enqueue with identical key should not create a duplicate row
	_, err = dispatcher.EnqueueNotification(key, "user", []string{"9876543210"}, "Test Title", "Test Body", map[string]string{"type": "tx"})
	if err != nil {
		t.Fatalf("duplicate enqueue failed: %v", err)
	}

	var count int64
	db.Model(&models.NotificationOutbox{}).Where("idempotency_key = ?", key).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly 1 row for idempotency_key, got %d", count)
	}
}

func TestNotificationDispatcher_WorkerCrashRecovery(t *testing.T) {
	db := setupDispatcherTestDB(t)
	outboxRepo := repositories.NewOutboxRepository(db)

	now := time.Now()
	// Insert job that was claimed 10 minutes ago and stuck in processing (simulating worker crash)
	staleLockedTime := now.Add(-10 * time.Minute)
	job := &models.NotificationOutbox{
		IdempotencyKey: "stale_job_1",
		RecipientType:  "user",
		TargetMobiles:  "[\"9876543210\"]",
		Title:          "Stale Alert",
		Body:           "Worker crashed",
		Status:         models.OutboxStatusProcessing,
		LockedAt:       &staleLockedTime,
		LockedBy:       "crashed-worker-pid-999",
		NextRetryAt:    now.Add(-15 * time.Minute),
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("failed to create stale job: %v", err)
	}

	// Recover stale locks with 2 minute timeout
	recovered, err := outboxRepo.RecoverStaleLocks(2 * time.Minute)
	if err != nil {
		t.Fatalf("RecoverStaleLocks failed: %v", err)
	}
	if recovered != 1 {
		t.Fatalf("expected 1 recovered job, got %d", recovered)
	}

	var updated models.NotificationOutbox
	db.First(&updated, job.ID)
	if updated.Status != models.OutboxStatusRetryableFailure {
		t.Fatalf("expected status to be retryable_failure, got %s", updated.Status)
	}
	if updated.LockedAt != nil || updated.LockedBy != "" {
		t.Fatalf("expected lock to be cleared, got locked_at=%v, locked_by=%s", updated.LockedAt, updated.LockedBy)
	}
}

func TestNotificationDispatcher_ContextCancellation(t *testing.T) {
	db := setupDispatcherTestDB(t)
	fcmService := NewFCMService(db)
	dispatcher := NewNotificationDispatcher(db, fcmService)

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	processed, err := dispatcher.ProcessNextBatch(ctx, 10)
	if err == nil && processed > 0 {
		t.Fatalf("expected canceled context error or zero processed, got processed=%d err=%v", processed, err)
	}
}

func TestNotificationDispatcher_CalculateBackoffWithJitter(t *testing.T) {
	b1 := CalculateBackoff(1, 5*time.Second, 10*time.Minute)
	if b1 < 5*time.Second || b1 > 10*time.Second {
		t.Fatalf("attempt 1 backoff out of expected range: %v", b1)
	}

	b3 := CalculateBackoff(3, 5*time.Second, 10*time.Minute)
	if b3 < 20*time.Second || b3 > 30*time.Second {
		t.Fatalf("attempt 3 backoff out of expected range: %v", b3)
	}

	bMax := CalculateBackoff(10, 5*time.Second, 10*time.Minute)
	if bMax > 13*time.Minute {
		t.Fatalf("backoff exceeded cap + jitter: %v", bMax)
	}
}
