package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"

	"split-udhar-apis/models"
	"split-udhar-apis/repositories"
	"split-udhar-apis/utils"

	"gorm.io/gorm"
)

// DispatchResult summarizes single job processing outcome.
type DispatchResult struct {
	JobID        uint
	SuccessCount int
	FailureCount int
	Accepted     bool
	Retryable    bool
	ErrorCode    string
	ErrorMessage string
}

// NotificationDispatcher coordinates asynchronous durable dispatching and background retries.
type NotificationDispatcher struct {
	db          *gorm.DB
	outboxRepo  repositories.OutboxRepository
	fcmService  *FCMService
	workerID    string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	isStarted   bool
	mu          sync.Mutex
	lockTimeout time.Duration
}

func NewNotificationDispatcher(db *gorm.DB, fcmService *FCMService) *NotificationDispatcher {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "worker"
	}
	workerID := fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano())

	return &NotificationDispatcher{
		db:          db,
		outboxRepo:  repositories.NewOutboxRepository(db),
		fcmService:  fcmService,
		workerID:    workerID,
		stopChan:    make(chan struct{}),
		lockTimeout: 2 * time.Minute,
	}
}

// EnqueueNotification saves an outbound notification into the durable MySQL outbox.
func (d *NotificationDispatcher) EnqueueNotification(
	idempotencyKey string,
	recipientType string,
	targetMobiles []string,
	title, body string,
	data map[string]string,
) (*models.NotificationOutbox, error) {
	mobilesJSON, err := json.Marshal(targetMobiles)
	if err != nil {
		return nil, fmt.Errorf("invalid recipient mobiles list: %w", err)
	}

	dataJSON, _ := json.Marshal(data)

	job := &models.NotificationOutbox{
		IdempotencyKey: idempotencyKey,
		RecipientType:  recipientType,
		TargetMobiles:  string(mobilesJSON),
		Title:          title,
		Body:           body,
		DataPayload:    string(dataJSON),
		Status:         models.OutboxStatusPending,
		Attempts:       0,
		MaxAttempts:    5,
		NextRetryAt:    time.Now(),
	}

	err = d.outboxRepo.Enqueue(job)
	if err != nil {
		return nil, err
	}

	// Trigger immediate async processing without blocking caller
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = d.ProcessNextBatch(ctx, 1)
	}()

	return job, nil
}

// CalculateBackoff computes bounded exponential backoff with full jitter:
// base * 2^attempts + jitter (capped at maxBackoff).
func CalculateBackoff(attempt int, baseBackoff, maxBackoff time.Duration) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	multiplier := math.Pow(2, float64(attempt-1))
	backoffSec := baseBackoff.Seconds() * multiplier

	if backoffSec > maxBackoff.Seconds() {
		backoffSec = maxBackoff.Seconds()
	}

	// Add random jitter up to 25% of backoff duration
	jitter := time.Duration(rand.Float64() * float64(time.Second) * (backoffSec * 0.25))
	return time.Duration(backoffSec)*time.Second + jitter
}

// ProcessJob delivers a single outbox notification job to FCM.
func (d *NotificationDispatcher) ProcessJob(ctx context.Context, job *models.NotificationOutbox) (*DispatchResult, error) {
	if job == nil {
		return nil, nil
	}

	var targetMobiles []string
	if err := json.Unmarshal([]byte(job.TargetMobiles), &targetMobiles); err != nil {
		_ = d.outboxRepo.MarkTerminalFailure(job.ID, "INVALID_MOBILES_JSON", err.Error(), job.Attempts+1)
		return &DispatchResult{JobID: job.ID, ErrorCode: "INVALID_MOBILES_JSON", ErrorMessage: err.Error()}, err
	}

	var data map[string]string
	if job.DataPayload != "" {
		_ = json.Unmarshal([]byte(job.DataPayload), &data)
	}

	newAttempts := job.Attempts + 1
	var sendErr error

	// Send through FCMService
	if job.RecipientType == "user" && len(targetMobiles) > 0 {
		sendErr = d.fcmService.SendNotificationToUser(targetMobiles[0], job.Title, job.Body, data)
	} else {
		sendErr = d.fcmService.SendNotificationToUsers(targetMobiles, job.Title, job.Body, data)
	}

	if sendErr == nil {
		_ = d.outboxRepo.MarkAccepted(job.ID, len(targetMobiles), 0)
		return &DispatchResult{JobID: job.ID, Accepted: true, SuccessCount: len(targetMobiles)}, nil
	}

	// Classify failure outcome
	cat, isRetryable, errCode := utils.ClassifyFCMError(sendErr)

	if !isRetryable || newAttempts >= job.MaxAttempts {
		log.Printf("[DISPATCHER] Notification job %d marked terminal failure (Category: %s, Attempts: %d/%d): %v",
			job.ID, cat, newAttempts, job.MaxAttempts, sendErr)
		_ = d.outboxRepo.MarkTerminalFailure(job.ID, errCode, sendErr.Error(), newAttempts)
		return &DispatchResult{
			JobID:        job.ID,
			Accepted:     false,
			Retryable:    false,
			ErrorCode:    errCode,
			ErrorMessage: sendErr.Error(),
		}, sendErr
	}

	// Calculate bounded exponential backoff with jitter
	backoff := CalculateBackoff(newAttempts, 5*time.Second, 10*time.Minute)
	nextRetryAt := time.Now().Add(backoff)

	log.Printf("[DISPATCHER] Notification job %d failed transiently (%s). Scheduling retry %d/%d in %v: %v",
		job.ID, errCode, newAttempts, job.MaxAttempts, backoff, sendErr)

	_ = d.outboxRepo.MarkRetryableFailure(job.ID, errCode, sendErr.Error(), nextRetryAt, newAttempts)
	return &DispatchResult{
		JobID:        job.ID,
		Accepted:     false,
		Retryable:    true,
		ErrorCode:    errCode,
		ErrorMessage: sendErr.Error(),
	}, sendErr
}

// ProcessNextBatch claims and executes a batch of pending outbox jobs.
func (d *NotificationDispatcher) ProcessNextBatch(ctx context.Context, batchSize int) (int, error) {
	// First recover any jobs left stranded by crashed workers
	_, _ = d.outboxRepo.RecoverStaleLocks(d.lockTimeout)

	jobs, err := d.outboxRepo.ClaimPendingJobs(d.workerID, batchSize, d.lockTimeout)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, job := range jobs {
		select {
		case <-ctx.Done():
			return processed, ctx.Err()
		default:
			j := job
			_, _ = d.ProcessJob(ctx, &j)
			processed++
		}
	}

	return processed, nil
}

// StartBackgroundWorker starts a periodic loop processing pending and retryable outbox items.
func (d *NotificationDispatcher) StartBackgroundWorker(pollInterval time.Duration) {
	d.mu.Lock()
	if d.isStarted {
		d.mu.Unlock()
		return
	}
	d.isStarted = true
	d.mu.Unlock()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-d.stopChan:
				log.Println("[DISPATCHER] Notification worker stopped.")
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
				_, _ = d.ProcessNextBatch(ctx, 10)
				cancel()
			}
		}
	}()

	log.Printf("[DISPATCHER] Background notification dispatcher worker started (poll: %v)", pollInterval)
}

// Stop stops the background worker cleanly.
func (d *NotificationDispatcher) Stop() {
	d.mu.Lock()
	if !d.isStarted {
		d.mu.Unlock()
		return
	}
	d.isStarted = false
	close(d.stopChan)
	d.mu.Unlock()

	d.wg.Wait()
}
