package repositories

import (
	"errors"
	"time"

	"split-udhar-apis/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutboxRepository interface {
	Enqueue(job *models.NotificationOutbox) error
	ClaimPendingJobs(workerID string, batchSize int, lockTimeout time.Duration) ([]models.NotificationOutbox, error)
	MarkAccepted(id uint, successCount, failureCount int) error
	MarkRetryableFailure(id uint, errCode, errMsg string, nextRetryAt time.Time, attempts int) error
	MarkTerminalFailure(id uint, errCode, errMsg string, attempts int) error
	RecoverStaleLocks(lockTimeout time.Duration) (int64, error)
	GetByID(id uint) (*models.NotificationOutbox, error)
	GetByIdempotencyKey(key string) (*models.NotificationOutbox, error)
}

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

// Enqueue saves a new notification job into the outbox.
// Uses OnConflict DoNothing on idempotency_key to prevent duplicate queuing.
func (r *outboxRepository) Enqueue(job *models.NotificationOutbox) error {
	if job == nil || job.IdempotencyKey == "" {
		return errors.New("valid job with idempotency_key is required")
	}
	if job.Status == "" {
		job.Status = models.OutboxStatusPending
	}
	if job.MaxAttempts == 0 {
		job.MaxAttempts = 5
	}
	if job.NextRetryAt.IsZero() {
		job.NextRetryAt = time.Now()
	}

	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(job).Error
}

// ClaimPendingJobs atomically locks and retrieves eligible jobs due for dispatch.
func (r *outboxRepository) ClaimPendingJobs(workerID string, batchSize int, lockTimeout time.Duration) ([]models.NotificationOutbox, error) {
	if batchSize <= 0 {
		batchSize = 10
	}
	now := time.Now()
	staleThreshold := now.Add(-lockTimeout)

	var claimedJobs []models.NotificationOutbox

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var candidateIDs []uint

		// Find candidates: pending or retryable, ready for retry, not actively locked
		err := tx.Model(&models.NotificationOutbox{}).
			Where("status IN ? AND next_retry_at <= ? AND (locked_at IS NULL OR locked_at < ?)",
				[]models.OutboxStatus{models.OutboxStatusPending, models.OutboxStatusRetryableFailure},
				now, staleThreshold).
			Order("next_retry_at ASC").
			Limit(batchSize).
			Pluck("id", &candidateIDs).Error

		if err != nil || len(candidateIDs) == 0 {
			return err
		}

		// Atomically update locked_at, locked_by and status to processing
		err = tx.Model(&models.NotificationOutbox{}).
			Where("id IN ?", candidateIDs).
			Updates(map[string]interface{}{
				"status":     models.OutboxStatusProcessing,
				"locked_at":  now,
				"locked_by":  workerID,
				"updated_at": now,
			}).Error

		if err != nil {
			return err
		}

		return tx.Where("id IN ?", candidateIDs).Find(&claimedJobs).Error
	})

	return claimedJobs, err
}

// MarkAccepted marks a job as accepted by FCM.
func (r *outboxRepository) MarkAccepted(id uint, successCount, failureCount int) error {
	now := time.Now()
	return r.db.Model(&models.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        models.OutboxStatusAccepted,
			"locked_at":     nil,
			"locked_by":     "",
			"success_count": successCount,
			"failure_count": failureCount,
			"updated_at":    now,
		}).Error
}

// MarkRetryableFailure updates job status with next backoff time and increments attempt count.
func (r *outboxRepository) MarkRetryableFailure(id uint, errCode, errMsg string, nextRetryAt time.Time, attempts int) error {
	now := time.Now()
	return r.db.Model(&models.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":          models.OutboxStatusRetryableFailure,
			"locked_at":       nil,
			"locked_by":       "",
			"attempts":        attempts,
			"last_error_code": errCode,
			"last_error_msg":  errMsg,
			"next_retry_at":   nextRetryAt,
			"updated_at":      now,
		}).Error
}

// MarkTerminalFailure marks a job as permanently failed after exhausting max attempts or terminal error.
func (r *outboxRepository) MarkTerminalFailure(id uint, errCode, errMsg string, attempts int) error {
	now := time.Now()
	return r.db.Model(&models.NotificationOutbox{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":          models.OutboxStatusFailed,
			"locked_at":       nil,
			"locked_by":       "",
			"attempts":        attempts,
			"last_error_code": errCode,
			"last_error_msg":  errMsg,
			"updated_at":      now,
		}).Error
}

// RecoverStaleLocks resets processing jobs that hung due to worker crashes.
func (r *outboxRepository) RecoverStaleLocks(lockTimeout time.Duration) (int64, error) {
	now := time.Now()
	staleThreshold := now.Add(-lockTimeout)

	res := r.db.Model(&models.NotificationOutbox{}).
		Where("status = ? AND locked_at < ?", models.OutboxStatusProcessing, staleThreshold).
		Updates(map[string]interface{}{
			"status":    models.OutboxStatusRetryableFailure,
			"locked_at": nil,
			"locked_by": "",
		})

	return res.RowsAffected, res.Error
}

func (r *outboxRepository) GetByID(id uint) (*models.NotificationOutbox, error) {
	var job models.NotificationOutbox
	err := r.db.First(&job, id).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *outboxRepository) GetByIdempotencyKey(key string) (*models.NotificationOutbox, error) {
	var job models.NotificationOutbox
	err := r.db.Where("idempotency_key = ?", key).First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}
