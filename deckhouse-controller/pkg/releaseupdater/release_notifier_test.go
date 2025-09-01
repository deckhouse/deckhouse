/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package releaseupdater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/go_lib/libapi"
)

func TestSendWebhookNotification(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		responseBody    string
		expectError     bool
		expectedError   string
		expectedRetries int
	}{
		{
			name:            "200 OK - should succeed",
			statusCode:      http.StatusOK,
			responseBody:    "Success",
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "201 Created - should succeed",
			statusCode:      http.StatusCreated,
			responseBody:    "Created",
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "299 Custom - should succeed",
			statusCode:      299,
			responseBody:    "Custom Success",
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "400 Bad Request - should fail after retries",
			statusCode:      http.StatusBadRequest,
			responseBody:    "Bad Request",
			expectError:     true,
			expectedError:   "webhook responded with status 400",
			expectedRetries: 5,
		},
		{
			name:            "404 Not Found - should fail after retries",
			statusCode:      http.StatusNotFound,
			responseBody:    "Not Found",
			expectError:     true,
			expectedError:   "webhook responded with status 404",
			expectedRetries: 5,
		},
		{
			name:            "500 Internal Server Error - should fail after retries",
			statusCode:      http.StatusInternalServerError,
			responseBody:    "Internal Server Error",
			expectError:     true,
			expectedError:   "webhook responded with status 500",
			expectedRetries: 5,
		},
		{
			name:            "300 Multiple Choices - should fail after retries",
			statusCode:      http.StatusMultipleChoices,
			responseBody:    "Multiple Choices",
			expectError:     true,
			expectedError:   "webhook responded with status 300",
			expectedRetries: 5,
		},
		{
			name:            "Large response body - should truncate",
			statusCode:      http.StatusNotFound,
			responseBody:    string(make([]byte, 10000)),
			expectError:     true,
			expectedError:   "webhook responded with status 404",
			expectedRetries: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attemptCount++
				w.WriteHeader(tt.statusCode)
				if _, err := w.Write([]byte(tt.responseBody)); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			}))
			defer svr.Close()

			config := NotificationConfig{
				WebhookURL:   svr.URL,
				RetryMinTime: libapi.Duration{Duration: 10 * time.Millisecond},
			}

			data := &WebhookData{
				Subject: "Test",
				Version: "1.0.0",
				Message: "Test message",
			}

			err := sendWebhookNotification(context.Background(), config, data)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Equal(t, tt.expectedRetries, attemptCount)

				// Check that error message includes response body (truncated if large)
				if len(tt.responseBody) > 4096 {
					// Error should be truncated, so it should be shorter than the full response
					assert.Less(t, len(err.Error()), len(tt.responseBody)+len(tt.expectedError)+len(": "))
				} else {
					assert.Contains(t, err.Error(), tt.responseBody)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedRetries, attemptCount)
			}
		})
	}
}

func TestSendWebhookNotification_RetryBehavior(t *testing.T) {
	t.Run("4 failures then success", func(t *testing.T) {
		attemptCount := 0
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 5 {
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte("Server Error")); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			} else {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("Success")); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			}
		}))
		defer svr.Close()

		config := NotificationConfig{
			WebhookURL:   svr.URL,
			RetryMinTime: libapi.Duration{Duration: 10 * time.Millisecond},
		}

		data := &WebhookData{
			Subject: "Test",
			Version: "1.0.0",
			Message: "Test message",
		}

		err := sendWebhookNotification(context.Background(), config, data)

		require.NoError(t, err)
		assert.Equal(t, 5, attemptCount)
	})

	t.Run("Network error then success", func(t *testing.T) {
		attemptCount := 0
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				// Simulate network error by closing connection
				hj, ok := w.(http.Hijacker)
				if ok {
					conn, _, _ := hj.Hijack()
					conn.Close()
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Success")); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		}))
		defer svr.Close()

		config := NotificationConfig{
			WebhookURL:   svr.URL,
			RetryMinTime: libapi.Duration{Duration: 10 * time.Millisecond},
		}

		data := &WebhookData{
			Subject: "Test",
			Version: "1.0.0",
			Message: "Test message",
		}

		err := sendWebhookNotification(context.Background(), config, data)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, attemptCount, 3)
	})
}

func TestSendWebhookNotification_DefaultRetryTime(t *testing.T) {
	t.Run("No RetryMinTime set - should use default", func(t *testing.T) {
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("Success")); err != nil {
				t.Fatalf("failed to write response: %v", err)
			}
		}))
		defer svr.Close()

		config := NotificationConfig{
			WebhookURL: svr.URL,
			// RetryMinTime not set - should use default 2 seconds
		}

		data := &WebhookData{
			Subject: "Test",
			Version: "1.0.0",
			Message: "Test message",
		}

		start := time.Now()
		err := sendWebhookNotification(context.Background(), config, data)
		duration := time.Since(start)

		require.NoError(t, err)
		// Should complete quickly since it succeeds on first try
		assert.Less(t, duration, 100*time.Millisecond)
	})

	t.Run("Custom RetryMinTime set", func(t *testing.T) {
		attemptCount := 0
		svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				if _, err := w.Write([]byte("Server Error")); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			} else {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("Success")); err != nil {
					t.Fatalf("failed to write response: %v", err)
				}
			}
		}))
		defer svr.Close()

		config := NotificationConfig{
			WebhookURL:   svr.URL,
			RetryMinTime: libapi.Duration{Duration: 50 * time.Millisecond},
		}

		data := &WebhookData{
			Subject: "Test",
			Version: "1.0.0",
			Message: "Test message",
		}

		start := time.Now()
		err := sendWebhookNotification(context.Background(), config, data)
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, 3, attemptCount)
		// Should take at least 50ms + 100ms (exponential backoff)
		assert.GreaterOrEqual(t, duration, 150*time.Millisecond)
	})
}

func TestWebhookError(t *testing.T) {
	t.Run("WebhookError structure and methods", func(t *testing.T) {
		webhookErr := &WebhookError{
			StatusCode: 404,
			Message:    "Not Found",
			Body:       "Resource not found",
		}

		errorMsg := webhookErr.Error()
		expectedMsg := "webhook responded with status 404: Not Found"
		assert.Equal(t, expectedMsg, errorMsg)

		jsonData, err := json.Marshal(webhookErr)
		require.NoError(t, err)

		var unmarshaled WebhookError
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, webhookErr.StatusCode, unmarshaled.StatusCode)
		assert.Equal(t, webhookErr.Message, unmarshaled.Message)
		assert.Equal(t, webhookErr.Body, unmarshaled.Body)
	})

	t.Run("WebhookError with empty body", func(t *testing.T) {
		webhookErr := &WebhookError{
			StatusCode: 500,
			Message:    "Internal Server Error",
		}

		errorMsg := webhookErr.Error()
		expectedMsg := "webhook responded with status 500: Internal Server Error"
		assert.Equal(t, expectedMsg, errorMsg)

		jsonData, err := json.Marshal(webhookErr)
		require.NoError(t, err)

		var unmarshaled WebhookError
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, webhookErr.StatusCode, unmarshaled.StatusCode)
		assert.Equal(t, webhookErr.Message, unmarshaled.Message)
		assert.Empty(t, unmarshaled.Body)
	})
}
