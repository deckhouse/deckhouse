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
	parts := splitN(authHeader, " ", 2)
	if len(parts) != 2 || !equalFold(parts[0], "Bearer") {
		return "", ErrInvalidAuthType
	}
	return parts[1], nil
}

func splitN(s, sep string, n int) []string {
	idx := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			idx = i
			break
		}
	}
	if idx == 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+1:]}
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
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

func TestMockVerifier_Verify(t *testing.T) {
	tests := []struct {
		name       string
		mockClaims *Claims
		mockErr    error
		wantErr    bool
	}{
		{
			name: "successful verification",
			mockClaims: &Claims{
				Username: "testuser",
				Email:    "test@example.com",
				Groups:   []string{"group1"},
			},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:       "verification error",
			mockClaims: nil,
			mockErr:    ErrTokenValidation,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &MockVerifier{
				claims: tt.mockClaims,
				err:    tt.mockErr,
			}

			claims, err := v.Verify(context.Background(), "sometoken")

			if (err != nil) != tt.wantErr {
				t.Errorf("Verify() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && claims.Username != tt.mockClaims.Username {
				t.Errorf("Verify() username = %v, want %v", claims.Username, tt.mockClaims.Username)
			}
		})
	}
}
