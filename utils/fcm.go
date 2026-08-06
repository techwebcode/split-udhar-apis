package utils

import (
	"bytes"
	"context"
	"encoding/json"
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
		invalidTokens, err := sendLegacyHTTPNotification([]string{token}, title, body, data)
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
			log.Printf("⚠️ FCM Stale Token Detected for token '%s...': %v", token[:min(10, len(token))], err)
			return true, err
		}
		log.Printf("❌ FCM Dispatch Error for token '%s...': %v", token[:min(10, len(token))], err)
		return false, err
	}

	log.Printf("🚀 FCM Sent Successfully to token '%s...'", token[:min(10, len(token))])
	return false, nil
}

// SendMulticastFCM sends push notifications to multiple device tokens.
// Returns list of invalid/unregistered tokens so repository can clean them up.
func SendMulticastFCM(tokens []string, title, body string, data map[string]string) ([]string, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	client := InitFCM()
	if client == nil {
		return sendLegacyHTTPNotification(tokens, title, body, data)
	}

	if data == nil {
		data = make(map[string]string)
	}
	data["click_action"] = "FLUTTER_NOTIFICATION_CLICK"

	multicastMsg := &messaging.MulticastMessage{
		Tokens: tokens,
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

	br, err := client.SendEachForMulticast(ctx, multicastMsg)
	if err != nil {
		log.Printf("❌ FCM Multicast Error: %v", err)
		return nil, err
	}

	var invalidTokens []string
	if br.FailureCount > 0 {
		for idx, resp := range br.Responses {
			if !resp.Success && resp.Error != nil {
				if messaging.IsUnregistered(resp.Error) || messaging.IsInvalidArgument(resp.Error) {
					invalidTokens = append(invalidTokens, tokens[idx])
				}
			}
		}
	}

	log.Printf("🚀 FCM Multicast Complete: %d Success, %d Failures (%d Invalid Tokens)", br.SuccessCount, br.FailureCount, len(invalidTokens))
	return invalidTokens, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RawDataJSON(data map[string]string) string {
	b, _ := json.Marshal(data)
	return string(b)
}

func sendLegacyHTTPNotification(tokens []string, title, body string, data map[string]string) ([]string, error) {
	serverKey := os.Getenv("FIREBASE_SERVICE_ACCOUNT_KEY")
	if serverKey == "" {
		serverKey = os.Getenv("FCM_SERVER_KEY")
	}
	if serverKey == "" || len(tokens) == 0 {
		log.Println("⚠️ FCM Notice: Service account / FCM key not configured, skipping notification")
		return nil, nil
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
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ FCM Legacy Dispatch Error: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	log.Printf("🚀 FCM Legacy HTTP Dispatch to %d tokens, status: %s", len(tokens), resp.Status)
	return nil, nil
}
