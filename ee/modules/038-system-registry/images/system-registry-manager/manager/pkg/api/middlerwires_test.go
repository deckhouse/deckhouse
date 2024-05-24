/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"time"
)

func TestSingleRequestMiddlewares(t *testing.T) {
	cfg := CreateSingleRequestConfig()

	// Handler that will be wrapped by SingleRequestMiddlewares
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create the middleware
	middleware := SingleRequestMiddlewares(handler, cfg)

	t.Run("allows first request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code, "expected status code 200")
		assert.Equal(t, "OK", rr.Body.String(), "expected body to be 'OK'")
	})

	t.Run("blocks subsequent request", func(t *testing.T) {
		done := make(chan struct{})
		go func() {
			// Lock the configuration to simulate a busy state
			cfg.tryToLock(1, 2)
			close(done)
		}()

		time.Sleep(100 * time.Millisecond) // Small sleep to ensure goroutine starts and acquires lock

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code, "expected status code 429")
		assert.Contains(t, rr.Body.String(), "Too Many Requests", "expected body to contain 'Too Many Requests'")

		<-done // Wait for the goroutine to finish
	})
}
