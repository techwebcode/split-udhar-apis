package utils

import (
	"strings"

	"firebase.google.com/go/v4/messaging"
)

// ErrorCategory classifies notification delivery failure outcomes.
type ErrorCategory string

const (
	ErrorCategoryNone            ErrorCategory = "NONE"
	ErrorCategoryPermanent       ErrorCategory = "PERMANENT_UNREGISTERED"
	ErrorCategoryInvalidArgument ErrorCategory = "INVALID_ARGUMENT"
	ErrorCategoryTransient       ErrorCategory = "TRANSIENT_RETRYABLE"
	ErrorCategoryQuota           ErrorCategory = "QUOTA_EXCEEDED"
	ErrorCategoryTerminal        ErrorCategory = "TERMINAL_FAILURE"
)

// ClassifyFCMError inspects an error returned by Firebase Messaging SDK
// and determines whether it is permanent or retryable.
func ClassifyFCMError(err error) (category ErrorCategory, isRetryable bool, code string) {
	if err == nil {
		return ErrorCategoryNone, false, "SUCCESS"
	}

	// 1. Permanent token invalidation / unregistration
	if messaging.IsUnregistered(err) {
		return ErrorCategoryPermanent, false, "UNREGISTERED"
	}
	if messaging.IsRegistrationTokenNotRegistered(err) {
		return ErrorCategoryPermanent, false, "TOKEN_NOT_REGISTERED"
	}
	if messaging.IsSenderIDMismatch(err) {
		return ErrorCategoryPermanent, false, "SENDER_ID_MISMATCH"
	}

	// 2. Client / argument errors (never retry bad arguments)
	if messaging.IsInvalidArgument(err) {
		return ErrorCategoryInvalidArgument, false, "INVALID_ARGUMENT"
	}

	// 3. Transient / retryable errors
	if messaging.IsServerUnavailable(err) || messaging.IsUnavailable(err) {
		return ErrorCategoryTransient, true, "SERVER_UNAVAILABLE"
	}
	if messaging.IsInternal(err) {
		return ErrorCategoryTransient, true, "INTERNAL_ERROR"
	}
	if messaging.IsQuotaExceeded(err) || messaging.IsMessageRateExceeded(err) {
		return ErrorCategoryQuota, true, "RATE_LIMITED"
	}

	// 4. String heuristics for standard HTTP/Network timeouts
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "credentials") ||
		strings.Contains(msg, "fcm disabled") ||
		strings.Contains(msg, "not initialized") {
		return ErrorCategoryTerminal, false, "CREDENTIALS_MISSING"
	}

	if strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "429") {
		return ErrorCategoryTransient, true, "NETWORK_TRANSIENT"
	}

	return ErrorCategoryTerminal, false, "UNKNOWN_ERROR"
}

// IsPermanentTokenFailure returns true if the token should be removed from database.
func IsPermanentTokenFailure(err error) bool {
	if err == nil {
		return false
	}
	cat, _, _ := ClassifyFCMError(err)
	return cat == ErrorCategoryPermanent
}

// IsTransientFCMError returns true if the failure should be scheduled for retry.
func IsTransientFCMError(err error) bool {
	if err == nil {
		return false
	}
	_, isRetryable, _ := ClassifyFCMError(err)
	return isRetryable
}
