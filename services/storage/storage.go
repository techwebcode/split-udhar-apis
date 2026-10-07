package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ImageStorageService defines the storage operations required for image management.
type ImageStorageService interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) error
	Delete(ctx context.Context, key string) error
	GetURL(key string) string
}

// R2StorageService implements ImageStorageService using Cloudflare R2's S3-compatible REST API.
type R2StorageService struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	PublicURL       string
	HTTPClient      *http.Client
}

// NewR2StorageService creates a new R2StorageService instance.
func NewR2StorageService(accountID, accessKeyID, secretAccessKey, bucketName, publicURL string) *R2StorageService {
	return &R2StorageService{
		AccountID:       strings.TrimSpace(accountID),
		AccessKeyID:     strings.TrimSpace(accessKeyID),
		SecretAccessKey: strings.TrimSpace(secretAccessKey),
		BucketName:      strings.TrimSpace(bucketName),
		PublicURL:       strings.TrimRight(strings.TrimSpace(publicURL), "/"),
		HTTPClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

// NewStorageServiceFromEnv initializes an ImageStorageService using environment variables.
// If R2 credentials are not set, it returns a MemoryStorageService so local dev and testing work seamlessly.
func NewStorageServiceFromEnv() ImageStorageService {
	accountID := os.Getenv("R2_ACCOUNT_ID")
	endpoint := os.Getenv("R2_ENDPOINT")
	if accountID == "" && endpoint != "" {
		// Extract account ID if full URL was provided in R2_ENDPOINT
		cleaned := strings.TrimPrefix(endpoint, "https://")
		cleaned = strings.TrimPrefix(cleaned, "http://")
		parts := strings.Split(cleaned, ".")
		if len(parts) > 0 {
			accountID = parts[0]
		}
	} else if strings.Contains(accountID, "http") || strings.Contains(accountID, ".") {
		cleaned := strings.TrimPrefix(accountID, "https://")
		cleaned = strings.TrimPrefix(cleaned, "http://")
		parts := strings.Split(cleaned, ".")
		if len(parts) > 0 {
			accountID = parts[0]
		}
	}

	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretKey := os.Getenv("R2_ACCESS_KEY_SECRET")
	if secretKey == "" {
		secretKey = os.Getenv("R2_SECRET_ACCESS_KEY")
	}
	bucketName := os.Getenv("R2_BUCKET_NAME")
	publicURL := os.Getenv("R2_PUBLIC_URL")

	if accountID != "" && accessKeyID != "" && secretKey != "" && bucketName != "" {
		return NewR2StorageService(accountID, accessKeyID, secretKey, bucketName, publicURL)
	}

	return NewMemoryStorageService(publicURL)
}

// Upload uploads an object to Cloudflare R2 using AWS SigV4 authorization.
func (r *R2StorageService) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	if r.BucketName == "" || r.AccountID == "" {
		return errors.New("R2 storage is not properly configured")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s/%s", r.AccountID, r.BucketName, escapeKey(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	if contentType == "" {
		contentType = "image/jpeg"
	}
	req.Header.Set("Content-Type", contentType)

	if err := r.signRequest(req, data); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("R2 upload network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("R2 upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// Delete removes an object from Cloudflare R2.
func (r *R2StorageService) Delete(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if r.BucketName == "" || r.AccountID == "" {
		return errors.New("R2 storage is not properly configured")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s/%s", r.AccountID, r.BucketName, escapeKey(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	if err := r.signRequest(req, nil); err != nil {
		return fmt.Errorf("failed to sign delete request: %w", err)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("R2 delete network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("R2 delete failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetURL returns the publicly accessible CDN URL for a given object key.
func (r *R2StorageService) GetURL(key string) string {
	if key == "" {
		return ""
	}
	if r.PublicURL != "" {
		return fmt.Sprintf("%s/%s", r.PublicURL, strings.TrimLeft(key, "/"))
	}
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s/%s", r.AccountID, r.BucketName, strings.TrimLeft(key, "/"))
}

func escapeKey(key string) string {
	parts := strings.Split(strings.TrimPrefix(key, "/"), "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

func (r *R2StorageService) signRequest(req *http.Request, payload []byte) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := sha256Hex(payload)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, payloadHash, amzDate)

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		req.Method,
		req.URL.Path,
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	)

	credentialScope := fmt.Sprintf("%s/auto/s3/aws4_request", dateStamp)
	stringToSign := fmt.Sprintf("AWS4-HMAC-SHA256\n%s\n%s\n%s",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	)

	signingKey := getSignatureKey(r.SecretAccessKey, dateStamp, "auto", "s3")
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		r.AccessKeyID, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)

	return nil
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func getSignatureKey(key, dateStamp, regionName, serviceName string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+key), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(regionName))
	kService := hmacSHA256(kRegion, []byte(serviceName))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	return kSigning
}

// MemoryStorageService is an in-memory implementation for testing and local development.
type MemoryStorageService struct {
	mu          sync.RWMutex
	Objects     map[string][]byte
	PublicURL   string
	SimulateErr error
}

func NewMemoryStorageService(publicURL string) *MemoryStorageService {
	if publicURL == "" {
		publicURL = "https://cdn.splitudhar.local"
	}
	return &MemoryStorageService{
		Objects:   make(map[string][]byte),
		PublicURL: strings.TrimRight(publicURL, "/"),
	}
}

func (m *MemoryStorageService) Upload(ctx context.Context, key string, data []byte, contentType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SimulateErr != nil {
		return m.SimulateErr
	}
	m.Objects[key] = append([]byte(nil), data...)
	return nil
}

func (m *MemoryStorageService) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SimulateErr != nil {
		return m.SimulateErr
	}
	delete(m.Objects, key)
	return nil
}

func (m *MemoryStorageService) GetURL(key string) string {
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", m.PublicURL, strings.TrimLeft(key, "/"))
}

func (m *MemoryStorageService) HasObject(key string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.Objects[key]
	return ok
}
