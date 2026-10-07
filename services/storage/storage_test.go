package storage

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestMemoryStorageService(t *testing.T) {
	mem := NewMemoryStorageService("https://pub.r2.dev")
	ctx := context.Background()

	data := []byte("test image content")
	key := "users/1/profile/avatar.jpg"
	contentType := "image/jpeg"

	// 1. Upload
	err := mem.Upload(ctx, key, data, contentType)
	if err != nil {
		t.Fatalf("unexpected upload error: %v", err)
	}

	// Verify URL
	url := mem.GetURL(key)
	expectedURL := "https://pub.r2.dev/users/1/profile/avatar.jpg"
	if url != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, url)
	}

	// Verify item exists
	if !mem.HasObject(key) {
		t.Errorf("expected key %q to exist in memory storage", key)
	}

	// 2. Delete
	err = mem.Delete(ctx, key)
	if err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	if mem.HasObject(key) {
		t.Errorf("expected key %q to be deleted", key)
	}

	// Delete non-existent key should not error
	err = mem.Delete(ctx, "nonexistent")
	if err != nil {
		t.Errorf("expected no error deleting non-existent key, got: %v", err)
	}
}

func TestR2StorageService_GetURL(t *testing.T) {
	// With PublicURL
	r2WithPublic := NewR2StorageService("acc123", "key123", "sec123", "bucket1", "https://cdn.splitudhar.com")
	url1 := r2WithPublic.GetURL("profiles/test.jpg")
	if url1 != "https://cdn.splitudhar.com/profiles/test.jpg" {
		t.Errorf("unexpected public URL: %s", url1)
	}

	// Empty key
	if r2WithPublic.GetURL("") != "" {
		t.Errorf("expected empty URL for empty key")
	}

	// Without PublicURL (falls back to R2 direct URL)
	r2Direct := NewR2StorageService("acc123", "key123", "sec123", "bucket1", "")
	url2 := r2Direct.GetURL("groups/1.png")
	expectedDirect := "https://acc123.r2.cloudflarestorage.com/bucket1/groups/1.png"
	if url2 != expectedDirect {
		t.Errorf("expected direct URL %q, got %q", expectedDirect, url2)
	}
}

func TestR2StorageService_SignRequest(t *testing.T) {
	r2 := NewR2StorageService("acc123", "key123", "sec123", "bucket1", "")
	payload := []byte("payload content")
	req, err := http.NewRequest(http.MethodPut, "https://acc123.r2.cloudflarestorage.com/bucket1/test.jpg", strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("failed to create http request: %v", err)
	}

	err = r2.signRequest(req, payload)
	if err != nil {
		t.Fatalf("failed to sign request: %v", err)
	}

	if req.Header.Get("x-amz-date") == "" {
		t.Errorf("missing x-amz-date header")
	}
	if req.Header.Get("x-amz-content-sha256") == "" {
		t.Errorf("missing x-amz-content-sha256 header")
	}
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=key123/") {
		t.Errorf("unexpected authorization header prefix: %s", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=") || !strings.Contains(auth, "Signature=") {
		t.Errorf("authorization header missing SignedHeaders or Signature: %s", auth)
	}
}
