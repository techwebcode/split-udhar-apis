package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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

// InitFCM initializes the Firebase Admin Messaging client.
func InitFCM() *messaging.Client {
	fcmInitOnce.Do(func() {
		ctx := context.Background()
		var opts []option.ClientOption

		credKey := os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY")
		credJSON := os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")

		if credJSON == "" && credKey != "" {
			credJSON = credKey
		}

		if credJSON != "" {
			if json.Valid([]byte(credJSON)) {
				opts = append(opts, option.WithCredentialsJSON([]byte(credJSON)))
				log.Println("🔥 FCM: Loaded credentials JSON from env")
			} else if _, err := os.Stat(credJSON); err == nil {
				opts = append(opts, option.WithCredentialsFile(credJSON))
				log.Printf("🔥 FCM: Loaded service account file from %s", credJSON)
			}
		}

		if len(opts) == 0 {
			candidates := []string{
				"firebase-service-account.json",
				"config/firebase-service-account.json",
				"../firebase-service-account.json",
				"../../firebase-service-account.json",
			}
			for _, path := range candidates {
				if _, err := os.Stat(path); err == nil {
					opts = append(opts, option.WithCredentialsFile(path))
					log.Printf("🔥 FCM: Loaded service account file from %s", path)
					break
				}
			}
		}

		if len(opts) > 0 {
			app, err := firebase.NewApp(ctx, nil, opts...)
			if err == nil {
				client, err := app.Messaging(ctx)
				if err == nil {
					fcmClient = client
					log.Println("✅ FCM: Firebase Cloud Messaging SDK initialized successfully")
					return
				}
			}
		}

		if credKey != "" {
			log.Println("🔥 FCM: FIREBASE_SERVICE_ACCOUNT_KEY configured for FCM notifications")
		} else {
			log.Println("⚠️ FCM Notice: Service account / key not configured yet")
		}
		fcmInitFailed = true
	})

	return fcmClient
}

// SendFCMNotification sends a push notification to a single device token.
// Returns (isInvalidToken, error).
func SendFCMNotification(token, title, body string, data map[string]string) (bool, error) {
	if token == "" {
		return false, nil
	}

	client := InitFCM()
	if client == nil {
		_, invalidTokens, err := sendLegacyHTTPNotification([]string{token}, title, body, data)
		return len(invalidTokens) > 0, err
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

// SendMulticastFCM sends push notifications to multiple device tokens.
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
		return sendLegacyHTTPNotification(tokens, title, body, data)
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

func sendLegacyHTTPNotification(tokens []string, title, body string, data map[string]string) (int, []string, error) {
	serverKey := os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY")
	if serverKey == "" {
		serverKey = os.Getenv("FCM_SERVER_KEY")
	}
	if serverKey == "" || len(tokens) == 0 {
		log.Println("⚠️ FCM Notice: Service account / FCM key not configured, skipping notification")
		return 0, nil, nil
	}

	url := "https://fcm.googleapis.com/fcm/send"

	if data == nil {
		data = make(map[string]string)
	}
	data["click_action"] = "FLUTTER_NOTIFICATION_CLICK"

	payload := map[string]interface{}{
		"registration_ids": tokens,
		"notification": map[string]interface{}{
			"title":      title,
			"body":       body,
			"sound":      "default",
			"channel_id": "split_udhar_notifications",
		},
		"data":     data,
		"priority": "high",
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ FCM Legacy Dispatch Error: %v", err)
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return len(tokens), nil, nil
	}
	return 0, nil, fmt.Errorf("FCM legacy endpoint responded with status %s", resp.Status)
}
