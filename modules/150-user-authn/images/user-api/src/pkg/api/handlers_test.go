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

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

type mockVerifier struct {
	claims *auth.Claims
	err    error
}

func (m *mockVerifier) Verify(_ context.Context, _ string) (*auth.Claims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func (m *mockVerifier) ExtractToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", auth.ErrNoAuthHeader
	}
	if len(authHeader) < 8 || authHeader[:7] != "Bearer " {
		return "", auth.ErrInvalidAuthType
	}
	return authHeader[7:], nil
}

type mockK8sClient struct {
	isLocal       bool
	isLocalErr    error
	operationName string
	createErr     error
}

func (m *mockK8sClient) IsLocalUser(_ context.Context, _ string) (bool, error) {
	return m.isLocal, m.isLocalErr
}

func (m *mockK8sClient) CreatePasswordResetOperation(_ context.Context, _, _ string) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	return m.operationName, nil
}

func (m *mockK8sClient) Start(_ context.Context) error {
	return nil
}

func (m *mockK8sClient) Stop() {}

func newTestHandler(verifier auth.Verifier, k8sClient k8s.Client) *Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewHandler(verifier, k8sClient, logger)
}

func TestHandler_Healthz(t *testing.T) {
	h := newTestHandler(&mockVerifier{}, &mockK8sClient{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.Healthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Healthz() status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Body.String() != "ok" {
		t.Errorf("Healthz() body = %q, want %q", w.Body.String(), "ok")
	}
}

func TestHandler_Readyz(t *testing.T) {
	h := newTestHandler(&mockVerifier{}, &mockK8sClient{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	h.Readyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Readyz() status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_ResetPassword(t *testing.T) {
	validBcryptHash := "$2a$10$fyCj7unkBxyxSrh6jULkTOwvUabNzdd81am02CT9tWTlb.y0Eptri"

	tests := []struct {
		name        string
		authHeader  string
		body        interface{}
		verifier    *mockVerifier
		k8sClient   *mockK8sClient
		wantStatus  int
		wantErrCode string
	}{
		{
			name:       "successful password reset",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser", Email: "test@example.com"},
			},
			k8sClient: &mockK8sClient{
				isLocal:       true,
				operationName: "self-password-reset-abc123",
			},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "missing auth header",
			authHeader: "",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier:   &mockVerifier{},
			k8sClient:  &mockK8sClient{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer invalidtoken",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier: &mockVerifier{
				err: auth.ErrTokenValidation,
			},
			k8sClient:  &mockK8sClient{},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing password hash",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: ""},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser"},
			},
			k8sClient:   &mockK8sClient{},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "invalid_request",
		},
		{
			name:       "invalid hash format - no prefix",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: "plaintext"},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser"},
			},
			k8sClient:   &mockK8sClient{},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "invalid_request",
		},
		{
			name:       "invalid hash format - wrong bcrypt structure",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: "$2y$10$invalidhash"},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser"},
			},
			k8sClient:   &mockK8sClient{},
			wantStatus:  http.StatusBadRequest,
			wantErrCode: "invalid_request",
		},
		{
			name:       "not a local user",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "externaluser"},
			},
			k8sClient: &mockK8sClient{
				isLocal: false,
			},
			wantStatus:  http.StatusForbidden,
			wantErrCode: "forbidden",
		},
		{
			name:       "k8s client error on local user check",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser"},
			},
			k8sClient: &mockK8sClient{
				isLocalErr: errors.New("k8s error"),
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "k8s client error on create operation",
			authHeader: "Bearer validtoken",
			body:       PasswordResetRequest{NewPasswordHash: validBcryptHash},
			verifier: &mockVerifier{
				claims: &auth.Claims{Username: "testuser"},
			},
			k8sClient: &mockK8sClient{
				isLocal:   true,
				createErr: k8s.ErrOperationFailed,
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.verifier, tt.k8sClient)

			bodyBytes, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/password/reset", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			h.ResetPassword(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ResetPassword() status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantErrCode != "" {
				var errResp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errResp.Error != tt.wantErrCode {
					t.Errorf("ResetPassword() error code = %q, want %q", errResp.Error, tt.wantErrCode)
				}
			}

			if tt.wantStatus == http.StatusAccepted {
				var resp PasswordResetResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode success response: %v", err)
				}
				if resp.Status != "accepted" {
					t.Errorf("ResetPassword() status = %q, want %q", resp.Status, "accepted")
				}
				if resp.OperationName == "" {
					t.Error("ResetPassword() operationName is empty")
				}
			}
		})
	}
}
