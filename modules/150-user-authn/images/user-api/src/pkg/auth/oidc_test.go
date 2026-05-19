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

package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockVerifier struct {
	claims *Claims
	err    error
}

func (m *MockVerifier) Verify(_ context.Context, _ string) (*Claims, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.claims, nil
}

func (m *MockVerifier) ExtractToken(r *http.Request) (string, error) {
	// Use the same case-insensitive logic as OIDCVerifier
	return extractBearerToken(r)
}

// extractBearerToken extracts Bearer token from Authorization header (shared logic)
func extractBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrNoAuthHeader
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthType
	}
	return parts[1], nil
}

func TestOIDCVerifier_ExtractToken(t *testing.T) {
	// Test the real OIDCVerifier.ExtractToken method
	verifier := &OIDCVerifier{}

	tests := []struct {
		name       string
		authHeader string
		wantToken  string
		wantErr    error
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer mytoken123",
			wantToken:  "mytoken123",
			wantErr:    nil,
		},
		{
			name:       "no header",
			authHeader: "",
			wantToken:  "",
			wantErr:    ErrNoAuthHeader,
		},
		{
			name:       "invalid type",
			authHeader: "Basic dXNlcjpwYXNz",
			wantToken:  "",
			wantErr:    ErrInvalidAuthType,
		},
		{
			name:       "bearer lowercase - should work (case insensitive)",
			authHeader: "bearer mytoken",
			wantToken:  "mytoken",
			wantErr:    nil,
		},
		{
			name:       "BEARER uppercase - should work (case insensitive)",
			authHeader: "BEARER mytoken",
			wantToken:  "mytoken",
			wantErr:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			token, err := verifier.ExtractToken(req)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ExtractToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("ExtractToken() token = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

