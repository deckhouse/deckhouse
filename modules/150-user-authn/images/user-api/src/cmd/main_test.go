/*
Copyright 2026 Flant JSC

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

package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"golang.org/x/time/rate"

	"user-api/pkg/api"
	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

// refusingVerifier refuses every token, so a request that reaches a self-service handler gets 401.
type refusingVerifier struct{}

func (refusingVerifier) Verify(_ context.Context, _ string) (*auth.Claims, error) {
	return nil, auth.ErrTokenValidation
}

func (refusingVerifier) ExtractToken(_ *http.Request) (string, error) {
	return "token", nil
}

type unusedK8sClient struct{ k8s.Client }

func testMux(limiter *rate.Limiter) *http.ServeMux {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return newMux(api.NewHandler(refusingVerifier{}, unusedK8sClient{}, logger), limiter)
}

func TestNewMux_Routes(t *testing.T) {
	mux := testMux(rate.NewLimiter(rate.Inf, 0))

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "self logout", method: http.MethodPost, path: "/api/v1/self/logout", wantStatus: http.StatusUnauthorized},
		{name: "self password change", method: http.MethodPost, path: "/api/v1/self/password-change", wantStatus: http.StatusUnauthorized},
		{name: "password reset is gone", method: http.MethodPost, path: "/api/v1/password/reset", wantStatus: http.StatusNotFound},
		{name: "self logout takes POST only", method: http.MethodGet, path: "/api/v1/self/logout", wantStatus: http.StatusMethodNotAllowed},
		{name: "devices", method: http.MethodGet, path: "/api/v1/self/sessions", wantStatus: http.StatusUnauthorized},
		{name: "logout of a device", method: http.MethodPost, path: "/api/v1/self/sessions/fehzwdh5qy4oyqcn7qhkcgzbm/logout", wantStatus: http.StatusUnauthorized},
		{name: "logout of a device takes POST only", method: http.MethodGet, path: "/api/v1/self/sessions/fehzwdh5qy4oyqcn7qhkcgzbm/logout", wantStatus: http.StatusMethodNotAllowed},
		{name: "logout of the other devices", method: http.MethodPost, path: "/api/v1/self/logout-others", wantStatus: http.StatusUnauthorized},
		{name: "health", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.wantStatus {
				t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, w.Code, tt.wantStatus)
			}
		})
	}
}

func TestNewMux_GlobalRateLimit(t *testing.T) {
	mux := testMux(rate.NewLimiter(rate.Limit(1), 1))

	for i, wantStatus := range []int{http.StatusUnauthorized, http.StatusTooManyRequests} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/self/logout", nil))
		if w.Code != wantStatus {
			t.Fatalf("request %d: status = %d, want %d", i, w.Code, wantStatus)
		}
		if wantStatus == http.StatusTooManyRequests && w.Header().Get("Retry-After") == "" {
			t.Error("a refused request has no Retry-After header")
		}
	}
}
