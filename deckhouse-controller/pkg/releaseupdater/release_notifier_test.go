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
	"fmt"
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
		contentLength   int64
		expectError     bool
		expectedError   string
		expectedRetries int
	}{
		{
			name:            "200 OK - should succeed",
			statusCode:      http.StatusOK,
			responseBody:    "Success",
			contentLength:   -1,
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "201 Created - should succeed",
			statusCode:      http.StatusCreated,
			responseBody:    "Created",
			contentLength:   -1,
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "299 Custom - should succeed",
			statusCode:      299,
			responseBody:    "Custom Success",
			contentLength:   -1,
			expectError:     false,
			expectedRetries: 1,
		},
		{
			name:            "400 Bad Request with JSON error - should parse error",
			statusCode:      http.StatusBadRequest,
			responseBody:    `{"code": "INVALID_DATA", "message": "Invalid request data"}`,
			contentLength:   60,
			expectError:     true,
			expectedError:   "webhook response with status code 400, service code: INVALID_DATA, msg: Invalid request data",
			expectedRetries: 5,
		},
		{
			name:            "400 Bad Request with JSON error without code - should parse error",
			statusCode:      http.StatusBadRequest,
			responseBody:    `{"message": "Invalid request data"}`,
			contentLength:   35,
			expectError:     true,
			expectedError:   "webhook response with status code 400, msg: Invalid request data",
			expectedRetries: 5,
		},
		{
			name:            "404 Not Found with no content - should return standard error",
			statusCode:      http.StatusNotFound,
			responseBody:    "",
			contentLength:   0,
			expectError:     true,
			expectedError:   "webhook response with status code 404",
			expectedRetries: 5,
		},
		{
			name:            "500 Internal Server Error with invalid JSON - should return standard error",
			statusCode:      http.StatusInternalServerError,
			responseBody:    `{"invalid": json}`,
			contentLength:   20,
			expectError:     true,
			expectedError:   "webhook response with status code 500",
			expectedRetries: 5,
		},
		{
			name:            "300 Multiple Choices - should fail after retries",
			statusCode:      http.StatusMultipleChoices,
			responseBody:    "Multiple Choices",
			contentLength:   -1,
			expectError:     true,
			expectedError:   "webhook response with status code 300",
			expectedRetries: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attemptCount++
				if tt.contentLength >= 0 {
					w.Header().Set("Content-Length", fmt.Sprintf("%d", tt.contentLength))
				}
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

		assert.GreaterOrEqual(t, duration, 150*time.Millisecond)
	})
}

func TestResponseError(t *testing.T) {
	t.Run("ResponseError JSON marshaling", func(t *testing.T) {
		resp := &ResponseError{
			Code:    "INVALID_DATA",
			Message: "Invalid request data",
		}

		jsonData, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ResponseError
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, resp.Code, unmarshaled.Code)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})

	t.Run("ResponseError without code", func(t *testing.T) {
		resp := &ResponseError{
			Message: "Service unavailable",
		}

		jsonData, err := json.Marshal(resp)
		require.NoError(t, err)

		var unmarshaled ResponseError
		err = json.Unmarshal(jsonData, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, "", unmarshaled.Code)
		assert.Equal(t, resp.Message, unmarshaled.Message)
	})
}
