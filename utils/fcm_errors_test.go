package utils

import (
	"errors"
	"testing"
)

func TestClassifyFCMError(t *testing.T) {
	// 1. Nil error
	cat, retryable, code := ClassifyFCMError(nil)
	if cat != ErrorCategoryNone || retryable || code != "SUCCESS" {
		t.Fatalf("expected None/false, got %v/%v", cat, retryable)
	}

	// 2. Network timeout
	timeoutErr := errors.New("post https://fcm.googleapis.com: context deadline exceeded (Client.Timeout exceeded)")
	cat, retryable, _ = ClassifyFCMError(timeoutErr)
	if cat != ErrorCategoryTransient || !retryable {
		t.Fatalf("expected Transient/true for timeout, got %v/%v", cat, retryable)
	}

	// 3. HTTP 503 Service Unavailable
	http503Err := errors.New("fcm server returned 503 Service Unavailable")
	cat, retryable, _ = ClassifyFCMError(http503Err)
	if cat != ErrorCategoryTransient || !retryable {
		t.Fatalf("expected Transient/true for 503, got %v/%v", cat, retryable)
	}

	// 4. Missing credentials configuration
	credErr := errors.New("FCM disabled: Firebase Admin SDK client not initialized (missing or invalid service account credentials)")
	cat, retryable, code = ClassifyFCMError(credErr)
	if cat != ErrorCategoryTerminal || retryable || code != "CREDENTIALS_MISSING" {
		t.Fatalf("expected Terminal/false/CREDENTIALS_MISSING for missing credentials, got %v/%v/%s", cat, retryable, code)
	}

	// 5. Unknown non-retryable error
	unknownErr := errors.New("unrecognized payload schema")
	cat, retryable, _ = ClassifyFCMError(unknownErr)
	if cat != ErrorCategoryTerminal || retryable {
		t.Fatalf("expected Terminal/false for unknown, got %v/%v", cat, retryable)
	}
}
