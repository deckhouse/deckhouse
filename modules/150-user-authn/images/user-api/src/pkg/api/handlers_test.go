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
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

var (
	janeClaims = &auth.Claims{
		Subject:     "CgRqYW5lEgVsb2NhbA",
		UserID:      "jane",
		ConnectorID: "local",
		Email:       "jane@example.com",
	}
	jdoeClaims = &auth.Claims{
		Subject:     "CiNjbj1qZG9lLG91PXBlb3BsZSxkYz1leGFtcGxlLGRjPW9yZxIEbGRhcA",
		UserID:      "cn=jdoe,ou=people,dc=example,dc=org",
		ConnectorID: "ldap",
		Email:       "jdoe@example.org",
	}
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
	token, found := strings.CutPrefix(authHeader, "Bearer ")
	if !found {
		return "", auth.ErrInvalidAuthType
	}
	return token, nil
}

type mockK8sClient struct {
	usernames   map[string]string
	usernameErr error
	createErr   error
	// statuses are returned by successive status reads; the last one repeats.
	statuses []k8s.OperationStatus

	mu      sync.Mutex
	created []k8s.UserOperation
	reads   int
}

func (m *mockK8sClient) LocalUsername(_ context.Context, userID string) (string, error) {
	if m.usernameErr != nil {
		return "", m.usernameErr
	}
	username, ok := m.usernames[userID]
	if !ok {
		return "", k8s.ErrUserNotFound
	}
	return username, nil
}

func (m *mockK8sClient) CreateUserOperation(_ context.Context, op k8s.UserOperation) (string, error) {
	if m.createErr != nil {
		return "", m.createErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.created = append(m.created, op)
	return op.GenerateName + "abcde", nil
}

func (m *mockK8sClient) UserOperationStatus(_ context.Context, _ string) (k8s.OperationStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.statuses) == 0 {
		return k8s.OperationStatus{}, errors.New("not found")
	}
	status := m.statuses[min(m.reads, len(m.statuses)-1)]
	m.reads++
	return status, nil
}

func (m *mockK8sClient) Start(_ context.Context) error {
	return nil
}

func (m *mockK8sClient) Stop() {}

func (m *mockK8sClient) createdOperations() []k8s.UserOperation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]k8s.UserOperation(nil), m.created...)
}

func newTestHandler(verifier auth.Verifier, k8sClient k8s.Client) *Handler {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	h := NewHandler(verifier, k8sClient, logger)
	h.waitTimeout = 200 * time.Millisecond
	h.pollInterval = time.Millisecond
	return h
}

// succeeded is the status of an operation the controller finished after one read.
func succeeded(message string) []k8s.OperationStatus {
	return []k8s.OperationStatus{{}, {Phase: k8s.PhaseSucceeded, Message: message}}
}

type selfRequestCase struct {
	name       string
	authHeader string
	body       string
	verifier   *mockVerifier
	k8sClient  *mockK8sClient
	wantStatus int
	// wantError is the error code of an error response; empty for a success response.
	wantError string
	// wantResponse is the status field of a success response.
	wantResponse string
	// wantOperation is the operation the request creates; nil when it creates none.
	wantOperation *k8s.UserOperation
	// wantMessage is the message of the response.
	wantMessage string
	// wantRetryAfter is the Retry-After header of the response; empty for none.
	wantRetryAfter string
}

func runSelfRequestCases(t *testing.T, endpoint string, handle func(*Handler) http.HandlerFunc, tests []selfRequestCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHandler(tt.verifier, tt.k8sClient)

			req := httptest.NewRequest(http.MethodPost, endpoint, strings.NewReader(tt.body))
			authHeader := tt.authHeader
			if authHeader == "" {
				authHeader = "Bearer validtoken"
			}
			if authHeader != "-" {
				req.Header.Set("Authorization", authHeader)
			}

			w := httptest.NewRecorder()
			handle(h)(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
			if got := w.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := w.Header().Get("Retry-After"); got != tt.wantRetryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tt.wantRetryAfter)
			}

			created := tt.k8sClient.createdOperations()
			switch {
			case tt.wantOperation == nil && len(created) != 0:
				t.Errorf("created %+v, want no operation", created)
			case tt.wantOperation != nil && (len(created) != 1 || !reflect.DeepEqual(created[0], *tt.wantOperation)):
				t.Errorf("created %+v, want %+v", created, *tt.wantOperation)
			}

			if tt.wantError != "" {
				var resp ErrorResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode error response: %v", err)
				}
				if resp.Error != tt.wantError {
					t.Errorf("error = %q, want %q", resp.Error, tt.wantError)
				}
				if tt.wantMessage != "" && resp.Message != tt.wantMessage {
					t.Errorf("message = %q, want %q", resp.Message, tt.wantMessage)
				}
				return
			}

			var resp OperationResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.Status != tt.wantResponse {
				t.Errorf("response status = %q, want %q", resp.Status, tt.wantResponse)
			}
			if want := tt.wantOperation.GenerateName + "abcde"; resp.OperationName != want {
				t.Errorf("operationName = %q, want %q", resp.OperationName, want)
			}
			if resp.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", resp.Message, tt.wantMessage)
			}
		})
	}
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

func TestHandler_SelfLogout(t *testing.T) {
	localLogout := &k8s.UserOperation{GenerateName: "self-logout-", Type: k8s.OperationTypeLogout, User: "jane"}
	externalLogout := &k8s.UserOperation{
		GenerateName: "self-logout-",
		Type:         k8s.OperationTypeLogout,
		Target: &k8s.Target{
			Email:       "jdoe@example.org",
			ConnectorID: "ldap",
			UserID:      "cn=jdoe,ou=people,dc=example,dc=org",
		},
	}
	janeStore := func(statuses []k8s.OperationStatus) *mockK8sClient {
		return &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: statuses}
	}

	runSelfRequestCases(t, logoutEndpoint, func(h *Handler) http.HandlerFunc { return h.SelfLogout }, []selfRequestCase{
		{
			name:          "local user is named by username",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     janeStore(succeeded("generation 4; 2 refresh tokens and 0 sessions deleted")),
			wantStatus:    http.StatusOK,
			wantResponse:  "done",
			wantOperation: localLogout,
			wantMessage:   "generation 4; 2 refresh tokens and 0 sessions deleted",
		},
		{
			name:          "external account is named by email, connector and user ID",
			verifier:      &mockVerifier{claims: jdoeClaims},
			k8sClient:     &mockK8sClient{statuses: succeeded("")},
			wantStatus:    http.StatusOK,
			wantResponse:  "done",
			wantOperation: externalLogout,
		},
		{
			name: "external account without an email",
			verifier: &mockVerifier{claims: &auth.Claims{
				Subject: jdoeClaims.Subject, UserID: jdoeClaims.UserID, ConnectorID: jdoeClaims.ConnectorID,
			}},
			k8sClient:  &mockK8sClient{},
			wantStatus: http.StatusForbidden,
			wantError:  "forbidden",
		},
		{
			name:       "local user without a Password",
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  &mockK8sClient{},
			wantStatus: http.StatusForbidden,
			wantError:  "forbidden",
		},
		{
			name:       "local user lookup error",
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  &mockK8sClient{usernameErr: k8s.ErrAmbiguousUser},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name:       "operation not created",
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  &mockK8sClient{usernames: map[string]string{"jane": "jane"}, createErr: k8s.ErrOperationFailed},
			wantStatus: http.StatusInternalServerError,
			wantError:  "internal_error",
		},
		{
			name:     "operation failed",
			verifier: &mockVerifier{claims: janeClaims},
			k8sClient: janeStore([]k8s.OperationStatus{
				{Phase: k8s.PhaseFailed, Message: "cannot find password for user: jane"},
			}),
			wantStatus:    http.StatusUnprocessableEntity,
			wantError:     "operation_failed",
			wantOperation: localLogout,
			wantMessage:   "cannot find password for user: jane",
		},
		{
			// m1: during the rollout the old controller leader fails a type it does not know. The
			// console retries a 503; a 422 would tell the user the logout is impossible.
			name:     "operation refused by a controller older than the type",
			verifier: &mockVerifier{claims: janeClaims},
			k8sClient: janeStore([]k8s.OperationStatus{
				{Phase: k8s.PhaseFailed, Message: "unsupported operation type: Logout"},
			}),
			wantStatus:     http.StatusServiceUnavailable,
			wantError:      "temporarily_unavailable",
			wantOperation:  localLogout,
			wantRetryAfter: "10",
		},
		{
			// Only the refusal of this very type means an old controller.
			name:     "operation failed with another unsupported type",
			verifier: &mockVerifier{claims: janeClaims},
			k8sClient: janeStore([]k8s.OperationStatus{
				{Phase: k8s.PhaseFailed, Message: "unsupported operation type: Lock"},
			}),
			wantStatus:    http.StatusUnprocessableEntity,
			wantError:     "operation_failed",
			wantOperation: localLogout,
			wantMessage:   "unsupported operation type: Lock",
		},
		{
			name:          "operation still running after the wait",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     janeStore([]k8s.OperationStatus{{}}),
			wantStatus:    http.StatusAccepted,
			wantResponse:  "pending",
			wantOperation: localLogout,
		},
		{
			name:          "status unreadable during the wait",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     janeStore(nil),
			wantStatus:    http.StatusAccepted,
			wantResponse:  "pending",
			wantOperation: localLogout,
		},
		{
			name:          "empty JSON object body",
			body:          "{}",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     janeStore(succeeded("")),
			wantStatus:    http.StatusOK,
			wantResponse:  "done",
			wantOperation: localLogout,
		},
		{
			// A client asking an older user-api to end one session must not sign out everywhere.
			name:       "body with a field this version does not know",
			body:       `{"session":"fehzwdh5qy4oyqcn7qhkcgzbm"}`,
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "body that is not an object",
			body:       `[]`,
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "body with trailing data",
			body:       `{} {}`,
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid_request",
		},
		{
			name:       "missing auth header",
			authHeader: "-",
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "token refused",
			verifier:   &mockVerifier{err: auth.ErrTokenValidation},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
		{
			name:       "subject that does not decode",
			verifier:   &mockVerifier{err: auth.ErrInvalidSubject},
			k8sClient:  janeStore(succeeded("")),
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
	})
}

func TestHandler_SelfPasswordChange(t *testing.T) {
	passwordChange := &k8s.UserOperation{
		GenerateName: "self-password-change-",
		Type:         k8s.OperationTypeRequirePasswordChange,
		User:         "jane",
	}

	runSelfRequestCases(t, passwordChangeEndpoint, func(h *Handler) http.HandlerFunc { return h.SelfPasswordChange }, []selfRequestCase{
		{
			name:          "local user",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: succeeded("")},
			wantStatus:    http.StatusOK,
			wantResponse:  "done",
			wantOperation: passwordChange,
		},
		{
			name: "Password found by user ID, not by the name claims",
			verifier: &mockVerifier{claims: &auth.Claims{
				Subject: "CgdqYW5lLWlkEgVsb2NhbA", UserID: "jane-id", ConnectorID: "local",
			}},
			k8sClient:     &mockK8sClient{usernames: map[string]string{"jane-id": "jane"}, statuses: succeeded("")},
			wantStatus:    http.StatusOK,
			wantResponse:  "done",
			wantOperation: passwordChange,
		},
		{
			name:          "operation still running after the wait",
			verifier:      &mockVerifier{claims: janeClaims},
			k8sClient:     &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: []k8s.OperationStatus{{}}},
			wantStatus:    http.StatusAccepted,
			wantResponse:  "pending",
			wantOperation: passwordChange,
		},
		{
			name:     "operation refused by a controller older than the type",
			verifier: &mockVerifier{claims: janeClaims},
			k8sClient: &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: []k8s.OperationStatus{
				{Phase: k8s.PhaseFailed, Message: "unsupported operation type: RequirePasswordChange"},
			}},
			wantStatus:     http.StatusServiceUnavailable,
			wantError:      "temporarily_unavailable",
			wantOperation:  passwordChange,
			wantRetryAfter: "10",
		},
		{
			// A federated user whose user ID matches a local one must not touch the local user.
			name:       "external account",
			verifier:   &mockVerifier{claims: &auth.Claims{Subject: "x", UserID: "jane", ConnectorID: "github"}},
			k8sClient:  &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: succeeded("")},
			wantStatus: http.StatusForbidden,
			wantError:  "forbidden",
		},
		{
			name:       "local user without a Password",
			verifier:   &mockVerifier{claims: janeClaims},
			k8sClient:  &mockK8sClient{},
			wantStatus: http.StatusForbidden,
			wantError:  "forbidden",
		},
		{
			name:       "token refused",
			verifier:   &mockVerifier{err: auth.ErrTokenValidation},
			k8sClient:  &mockK8sClient{usernames: map[string]string{"jane": "jane"}},
			wantStatus: http.StatusUnauthorized,
			wantError:  "unauthorized",
		},
	})
}

func TestHandler_UserRateLimit(t *testing.T) {
	k8sClient := &mockK8sClient{
		usernames: map[string]string{"jane": "jane", "john": "john"},
		statuses:  succeeded(""),
	}
	verifier := &mockVerifier{claims: janeClaims}
	h := newTestHandler(verifier, k8sClient)

	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return now }

	call := func(handle http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.Header.Set("Authorization", "Bearer validtoken")
		w := httptest.NewRecorder()
		handle(w, req)
		return w
	}

	// The burst of three is shared by both endpoints.
	for i, handle := range []http.HandlerFunc{h.SelfLogout, h.SelfPasswordChange, h.SelfLogout} {
		if w := call(handle); w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d, body = %s", i, w.Code, http.StatusOK, w.Body.String())
		}
	}

	w := call(h.SelfPasswordChange)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("fourth request: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if got := w.Header().Get("Retry-After"); got != "10" {
		t.Errorf("Retry-After = %q, want 10", got)
	}
	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error != "rate_limit_exceeded" {
		t.Errorf("error = %q, want rate_limit_exceeded", resp.Error)
	}
	if got := len(k8sClient.createdOperations()); got != 3 {
		t.Errorf("created %d operations, want 3: a refused request creates none", got)
	}

	// Another user has a budget of their own.
	verifier.claims = &auth.Claims{Subject: "CgRqb2huEgVsb2NhbA", UserID: "john", ConnectorID: "local"}
	if w := call(h.SelfLogout); w.Code != http.StatusOK {
		t.Fatalf("other user: status = %d, want %d", w.Code, http.StatusOK)
	}

	// One request comes back every 10 seconds.
	verifier.claims = janeClaims
	now = now.Add(4 * time.Second)
	w = call(h.SelfLogout)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("after 4 seconds: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
	if got := w.Header().Get("Retry-After"); got != "6" {
		t.Errorf("after 4 seconds: Retry-After = %q, want 6", got)
	}
	now = now.Add(6 * time.Second)
	if w := call(h.SelfLogout); w.Code != http.StatusOK {
		t.Fatalf("after 10 seconds: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHandler_ReturnsWhenTheClientGoesAway(t *testing.T) {
	k8sClient := &mockK8sClient{usernames: map[string]string{"jane": "jane"}, statuses: []k8s.OperationStatus{{}}}
	h := newTestHandler(&mockVerifier{claims: janeClaims}, k8sClient)
	h.waitTimeout = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, logoutEndpoint, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer validtoken")

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.SelfLogout(httptest.NewRecorder(), req)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler kept waiting after the request context was canceled")
	}
}
