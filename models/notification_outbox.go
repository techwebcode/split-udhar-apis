package models

import (
	"time"

	"gorm.io/gorm"
)

type OutboxStatus string

const (
	OutboxStatusPending          OutboxStatus = "pending"
	OutboxStatusProcessing       OutboxStatus = "processing"
	OutboxStatusAccepted         OutboxStatus = "accepted"
	OutboxStatusRetryableFailure OutboxStatus = "retryable_failure"
	OutboxStatusFailed           OutboxStatus = "failed"
)

// NotificationOutbox holds push notifications for resilient asynchronous delivery.
type NotificationOutbox struct {
	ID             uint         `gorm:"primaryKey" json:"id"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	
	IdempotencyKey string       `gorm:"size:100;uniqueIndex;not null" json:"idempotency_key"`
	RecipientType  string       `gorm:"size:20;not null" json:"recipient_type"` // "user" or "users"
	TargetMobiles  string       `gorm:"type:text;not null" json:"target_mobiles"` // JSON array string
	Title          string       `gorm:"size:255;not null" json:"title"`
	Body           string       `gorm:"size:500;not null" json:"body"`
	DataPayload    string       `gorm:"type:text" json:"data_payload"` // JSON string map[string]string
	
	Status         OutboxStatus `gorm:"size:30;index;default:'pending';not null" json:"status"`
	Attempts       int          `gorm:"default:0;not null" json:"attempts"`
	MaxAttempts    int          `gorm:"default:5;not null" json:"max_attempts"`
	NextRetryAt    time.Time    `gorm:"index;not null" json:"next_retry_at"`
	LockedAt       *time.Time   `gorm:"index" json:"locked_at"`
	LockedBy       string       `gorm:"size:100" json:"locked_by"`
	
	LastErrorCode  string       `gorm:"size:100" json:"last_error_code"`
	LastErrorMsg   string       `gorm:"type:text" json:"last_error_msg"`
	SuccessCount   int          `gorm:"default:0" json:"success_count"`
	FailureCount   int          `gorm:"default:0" json:"failure_count"`
}
