package utils

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

var (
	fcmClient     *messaging.Client
	fcmInitOnce   sync.Once
	fcmInitFailed bool
)

// MaskToken produces a privacy-safe truncated version of an FCM token for structured logging.
func MaskToken(token string) string {
	if len(token) <= 14 {
		return "[redacted]"
	}
	return fmt.Sprintf("%s...%s", token[:10], token[len(token)-4:])
}

// InitFCM initializes the Firebase Admin Messaging client using HTTP v1.
func InitFCM() *messaging.Client {
	fcmInitOnce.Do(func() {
		ctx := context.Background()
		var opts []option.ClientOption

		// 1. Explicit file path environment variables
		filePathCandidates := []string{
			os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE"),
			os.Getenv("FIREBASE_SERVICE_ACCOUNT_PATH"),
			os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		}
		for _, fp := range filePathCandidates {
			if fp != "" {
				if _, err := os.Stat(fp); err == nil {
					opts = append(opts, option.WithCredentialsFile(fp))
					log.Printf("🔥 FCM: Loaded service account file from %s", fp)
					break
				}
			}
		}

		// 2. Direct JSON, Base64, or file path in FIREBASE_SERVICE_ACCOUNT_KEY / JSON
		if len(opts) == 0 {
			rawCreds := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")
			if rawCreds == "" {
				rawCreds = os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY")
			}

			if rawCreds != "" {
				if _, err := os.Stat(rawCreds); err == nil {
					// File path provided in env
					opts = append(opts, option.WithCredentialsFile(rawCreds))
					log.Printf("🔥 FCM: Loaded service account file from path in env: %s", rawCreds)
				} else if json.Valid([]byte(rawCreds)) {
					// Raw JSON string
					opts = append(opts, option.WithCredentialsJSON([]byte(rawCreds)))
					log.Println("🔥 FCM: Loaded service account JSON directly from environment")
				} else if decoded, err := base64.StdEncoding.DecodeString(rawCreds); err == nil && json.Valid(decoded) {
					// Base64-encoded JSON string
					opts = append(opts, option.WithCredentialsJSON(decoded))
					log.Println("🔥 FCM: Loaded base64-decoded service account JSON from environment")
				} else {
					log.Printf("⚠️ FCM Warning: Credential in env is neither valid JSON nor existing file (len=%d). Note: FCM legacy API was shut down by Google in June 2024; legacy server keys cannot be used with FCM HTTP v1. Please provide service account JSON or file path.", len(rawCreds))
				}
			}
		}

		// 3. Fallback candidate files in common project locations
		if len(opts) == 0 {
			var candidatePaths []string
			// Relative to working directory
			candidatePaths = append(candidatePaths,
				"firebase-service-account.json",
				"config/firebase-service-account.json",
				"../firebase-service-account.json",
				"../../firebase-service-account.json",
			)
			// Relative to running executable directory (critical for production systemd services)
			if execPath, err := os.Executable(); err == nil {
				execDir := filepath.Dir(execPath)
				candidatePaths = append(candidatePaths,
					filepath.Join(execDir, "firebase-service-account.json"),
					filepath.Join(execDir, "config", "firebase-service-account.json"),
				)
			}
			// System locations
			candidatePaths = append(candidatePaths,
				"/etc/splitudhar/firebase-service-account.json",
			)

			for _, path := range candidatePaths {
				if _, err := os.Stat(path); err == nil {
					opts = append(opts, option.WithCredentialsFile(path))
					log.Printf("🔥 FCM: Discovered service account file at %s", path)
					break
				}
			}
		}

		// 4. Initialize Firebase App with options
		if len(opts) > 0 {
			app, err := firebase.NewApp(ctx, nil, opts...)
			if err != nil {
				log.Printf("❌ FCM Init Error: Failed to create Firebase app: %v", err)
				fcmInitFailed = true
				return
			}
			client, err := app.Messaging(ctx)
			if err != nil {
				log.Printf("❌ FCM Init Error: Failed to initialize Firebase messaging client: %v", err)
				fcmInitFailed = true
				return
			}
			fcmClient = client
			log.Println("✅ FCM: Firebase Cloud Messaging SDK (HTTP v1) initialized successfully")
			return
		}

		// 5. Try Google Application Default Credentials (e.g. GCP GCE instance metadata)
		app, err := firebase.NewApp(ctx, nil)
		if err == nil {
			client, err := app.Messaging(ctx)
			if err == nil {
				fcmClient = client
				log.Println("✅ FCM: Firebase Cloud Messaging SDK initialized via Google Application Default Credentials")
				return
			}
		}

		log.Println("⚠️ FCM Notice: No valid Firebase service account credentials found. Push notifications will be skipped.")
		fcmInitFailed = true
	})

	return fcmClient
}

// SendFCMNotification sends a push notification to a single device token using HTTP v1.
// Returns (isInvalidToken, error).
func SendFCMNotification(token, title, body string, data map[string]string) (bool, error) {
	if token == "" {
		return false, nil
	}

	client := InitFCM()
	if client == nil {
		return false, fmt.Errorf("FCM disabled: Firebase Admin SDK client not initialized (missing or invalid service account credentials)")
	}

	if data == nil {
		data = make(map[string]string)
	}
	data["click_action"] = "FLUTTER_NOTIFICATION_CLICK"

	msg := &messaging.Message{
		Token: token,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				Sound:       "default",
				ClickAction: "FLUTTER_NOTIFICATION_CLICK",
				ChannelID:   "split_udhar_notifications",
			},
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					Sound: "default",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.Send(ctx, msg)
	if err != nil {
		if messaging.IsUnregistered(err) || messaging.IsInvalidArgument(err) {
			log.Printf("⚠️ [FCM] Stale Token Detected for token '%s': %v", MaskToken(token), err)
			return true, err
		}
		log.Printf("❌ [FCM] Dispatch Error for token '%s': %v", MaskToken(token), err)
		return false, err
	}

	log.Printf("🚀 [FCM] Message accepted by FCM for token '%s'", MaskToken(token))
	return false, nil
}

// SendMulticastFCM sends push notifications to multiple device tokens using HTTP v1.
// Automatically chunks requests to respect Firebase 500-token batch limits.
// Returns (successfulDeliveriesCount, invalidTokensList, error).
func SendMulticastFCM(tokens []string, title, body string, data map[string]string) (int, []string, error) {
	if len(tokens) == 0 {
		return 0, nil, nil
	}

	// 1. Deduplicate tokens
	tokens = DeduplicateTokens(tokens)
	if len(tokens) == 0 {
		return 0, nil, nil
	}

	client := InitFCM()
	if client == nil {
		return 0, nil, fmt.Errorf("FCM disabled: Firebase Admin SDK client not initialized (missing or invalid service account credentials)")
	}

	if data == nil {
		data = make(map[string]string)
	}
	data["click_action"] = "FLUTTER_NOTIFICATION_CLICK"

	const maxBatchSize = 500
	var totalSuccess int
	var allInvalidTokens []string
	var transientErrors []error

	for i := 0; i < len(tokens); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		batchTokens := tokens[i:end]

		multicastMsg := &messaging.MulticastMessage{
			Tokens: batchTokens,
			Notification: &messaging.Notification{
				Title: title,
				Body:  body,
			},
			Data: data,
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					Sound:       "default",
					ClickAction: "FLUTTER_NOTIFICATION_CLICK",
					ChannelID:   "split_udhar_notifications",
				},
			},
			APNS: &messaging.APNSConfig{
				Payload: &messaging.APNSPayload{
					Aps: &messaging.Aps{
						Sound: "default",
					},
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		br, err := client.SendEachForMulticast(ctx, multicastMsg)
		cancel()

		if err != nil {
			log.Printf("❌ [FCM] Multicast batch failed for %d tokens: %v", len(batchTokens), err)
			transientErrors = append(transientErrors, err)
			continue
		}

		totalSuccess += br.SuccessCount
		if br.FailureCount > 0 {
			for idx, resp := range br.Responses {
				if !resp.Success && resp.Error != nil {
					if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
						allInvalidTokens = append(allInvalidTokens, batchTokens[idx])
					} else {
						transientErrors = append(transientErrors, resp.Error)
					}
				}
			}
		}
	}

	log.Printf("🚀 [FCM] Multicast Result: %d Attempted, %d Accepted by FCM, %d Permanent Invalid, %d Transient Failures",
		len(tokens), totalSuccess, len(allInvalidTokens), len(transientErrors))

	var returnErr error
	if totalSuccess == 0 && len(transientErrors) > 0 {
		returnErr = transientErrors[0]
	}

	return totalSuccess, allInvalidTokens, returnErr
}

// DeduplicateTokens removes duplicate or blank tokens while preserving order.
func DeduplicateTokens(tokens []string) []string {
	seen := make(map[string]bool)
	var deduped []string
	for _, token := range tokens {
		if token != "" && !seen[token] {
			seen[token] = true
			deduped = append(deduped, token)
		}
	}
	return deduped
}
