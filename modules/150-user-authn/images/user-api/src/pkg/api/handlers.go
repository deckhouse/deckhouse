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
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

const (
	logoutEndpoint         = "/api/v1/self/logout"
	passwordChangeEndpoint = "/api/v1/self/password-change"

	// The body of a self-service request is an empty JSON object. A few bytes are enough to tell
	// one from anything else.
	maxRequestBodySize = 4 << 10

	// operationWaitTimeout bounds how long a request waits for the controller to finish the
	// operation; past it the client gets 202 and the operation goes on.
	operationWaitTimeout  = 10 * time.Second
	operationPollInterval = 250 * time.Millisecond

	// unsupportedTypeMessage starts the message a user-authn-controller older than the Logout and
	// RequirePasswordChange types fails them with (useroperation/execute.go). While the controller
	// is rolled out the old leader can take an operation before the new one leads; the operation
	// is fine, the controller is not ready for it yet.
	unsupportedTypeMessage = "unsupported operation type: "
	// retryAfterControllerUpdate is the Retry-After of the answer to such an operation, in seconds:
	// the new controller leads within seconds, and the budget of the user allows another request.
	retryAfterControllerUpdate = "10"
)

// localDexConnectorID is the connector ID Dex assigns to tokens issued by its
// built-in local password database (enablePasswordDB: true). A password change is only meaningful
// for those users; tokens minted by external connectors (LDAP, OIDC, GitHub, ...) carry a different
// connector ID.
const localDexConnectorID = "local"

// Values of OperationResponse.Status.
const (
	statusDone    = "done"
	statusPending = "pending"
)

type Handler struct {
	verifier   auth.Verifier
	k8sClient  k8s.Client
	logger     *slog.Logger
	ready      atomic.Bool
	registry   *prometheus.Registry
	reqCounter *prometheus.CounterVec

	userLimiter  *userLimiter
	now          func() time.Time
	waitTimeout  time.Duration
	pollInterval time.Duration
}

// OperationResponse reports the UserOperation a request created: "done" once the controller has
// carried it out, "pending" while it has not yet.
type OperationResponse struct {
	Status        string `json:"status"`
	OperationName string `json:"operationName"`
	Message       string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	// OperationName is set when the controller refused the operation the request created.
	OperationName string `json:"operationName,omitempty"`
}

// selfRequest is the body of a self-service request. It has no fields yet; unknown ones are refused,
// so that a client sending a narrower request to an older user-api does not get a wider action.
type selfRequest struct{}

func NewHandler(verifier auth.Verifier, k8sClient k8s.Client, logger *slog.Logger) *Handler {
	registry := prometheus.NewRegistry()

	reqCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "user_api_requests_total",
		Help: "Total number of requests by endpoint and status",
	}, []string{"endpoint", "status"})

	registry.MustRegister(reqCounter)
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	h := &Handler{
		verifier:     verifier,
		k8sClient:    k8sClient,
		logger:       logger,
		registry:     registry,
		reqCounter:   reqCounter,
		userLimiter:  newUserLimiter(userRequestInterval, userRequestBurst),
		now:          time.Now,
		waitTimeout:  operationWaitTimeout,
		pollInterval: operationPollInterval,
	}
	h.ready.Store(true)

	return h
}

func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *Handler) Readyz(w http.ResponseWriter, _ *http.Request) {
	if h.ready.Load() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready"))
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// SelfLogout signs the user out everywhere: a UserOperation Logout revokes every token, session and
// kubeconfig of the pair the token names. A local user is named by username, the account of an
// external provider by its email, connector ID and user ID.
func (h *Handler) SelfLogout(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.beginSelfRequest(w, r, logoutEndpoint)
	if !ok {
		return
	}

	op := k8s.UserOperation{
		GenerateName: "self-logout-",
		Type:         k8s.OperationTypeLogout,
	}

	if claims.ConnectorID == localDexConnectorID {
		username, ok := h.localUsername(w, r, logoutEndpoint, claims)
		if !ok {
			return
		}
		op.User = username
	} else {
		// The controller finds an external account by its email, which the provider reported at
		// sign-in; without one there is nothing to name the account by.
		if claims.Email == "" {
			h.logger.Warn("Self logout refused: the token has no email",
				"user_id", claims.UserID, "connector_id", claims.ConnectorID, "remote_addr", r.RemoteAddr)
			h.fail(w, logoutEndpoint, http.StatusForbidden, "forbidden",
				"Signing out everywhere needs an email in the token")
			return
		}
		op.Target = &k8s.Target{
			Email:       claims.Email,
			ConnectorID: claims.ConnectorID,
			UserID:      claims.UserID,
		}
	}

	h.runOperation(w, r, logoutEndpoint, claims, op)
}

// SelfPasswordChange asks a local user to change the password at the next sign-in: a UserOperation
// RequirePasswordChange sets the flag on the Password object, and Dex asks for the current and a new
// password, under the password policy, at the next interactive sign-in.
func (h *Handler) SelfPasswordChange(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.beginSelfRequest(w, r, passwordChangeEndpoint)
	if !ok {
		return
	}

	// Only the built-in password database has passwords Dex can change. The connector ID comes
	// from the signed subject, so a user of another provider cannot claim a local username.
	if claims.ConnectorID != localDexConnectorID {
		h.logger.Warn("Password change attempted with non-local connector token",
			"user_id", claims.UserID, "connector_id", claims.ConnectorID, "remote_addr", r.RemoteAddr)
		h.fail(w, passwordChangeEndpoint, http.StatusForbidden, "forbidden",
			"Password change is only available for local users")
		return
	}

	username, ok := h.localUsername(w, r, passwordChangeEndpoint, claims)
	if !ok {
		return
	}

	h.runOperation(w, r, passwordChangeEndpoint, claims, k8s.UserOperation{
		GenerateName: "self-password-change-",
		Type:         k8s.OperationTypeRequirePasswordChange,
		User:         username,
	})
}

// beginSelfRequest authenticates the request, checks its body and takes it from the budget of the
// user. It writes the response and returns false when the request goes no further.
func (h *Handler) beginSelfRequest(w http.ResponseWriter, r *http.Request, endpoint string) (*auth.Claims, bool) {
	token, err := h.verifier.ExtractToken(r)
	if err != nil {
		h.logger.Warn("Failed to extract token", "error", err, "remote_addr", r.RemoteAddr)
		h.fail(w, endpoint, http.StatusUnauthorized, "unauthorized", "Missing or invalid authorization header")
		return nil, false
	}

	claims, err := h.verifier.Verify(r.Context(), token)
	if err != nil {
		h.logger.Warn("Failed to verify token", "error", err, "remote_addr", r.RemoteAddr)
		h.fail(w, endpoint, http.StatusUnauthorized, "unauthorized", "Invalid token")
		return nil, false
	}

	if err := readSelfRequest(w, r); err != nil {
		h.logger.Warn("Invalid request body", "error", err, "user_id", claims.UserID, "connector_id", claims.ConnectorID)
		h.fail(w, endpoint, http.StatusBadRequest, "invalid_request", "The request body must be empty or an empty JSON object")
		return nil, false
	}

	if delay := h.userLimiter.reserve(claims.Subject, h.now()); delay > 0 {
		h.logger.Warn("Rate limit exceeded for user", "user_id", claims.UserID, "connector_id", claims.ConnectorID)
		w.Header().Set("Retry-After", retryAfterSeconds(delay))
		h.fail(w, endpoint, http.StatusTooManyRequests, "rate_limit_exceeded", "Too many requests, please try again later")
		return nil, false
	}

	return claims, true
}

// localUsername finds the username of a local user. It writes the response and returns false when
// there is none.
func (h *Handler) localUsername(w http.ResponseWriter, r *http.Request, endpoint string, claims *auth.Claims) (string, bool) {
	username, err := h.k8sClient.LocalUsername(r.Context(), claims.UserID)
	switch {
	case err == nil:
		return username, true
	case errors.Is(err, k8s.ErrUserNotFound):
		h.logger.Warn("No local user has the user ID of the token", "user_id", claims.UserID, "remote_addr", r.RemoteAddr)
		h.fail(w, endpoint, http.StatusForbidden, "forbidden", "The local user of this token does not exist")
	default:
		h.logger.Error("Failed to find local user", "error", err, "user_id", claims.UserID)
		h.fail(w, endpoint, http.StatusInternalServerError, "internal_error", "Failed to verify user")
	}
	return "", false
}

// runOperation creates op and waits for the controller to finish it: 200 when it succeeded, 422
// when it failed, 202 when it is still running after the wait. An operation a controller older
// than its type refused is answered 503 with Retry-After: the request is right, and it succeeds
// once the new controller leads.
func (h *Handler) runOperation(w http.ResponseWriter, r *http.Request, endpoint string, claims *auth.Claims, op k8s.UserOperation) {
	name, err := h.k8sClient.CreateUserOperation(r.Context(), op)
	if err != nil {
		h.logger.Error("Failed to create user operation", "error", err, "type", op.Type,
			"user_id", claims.UserID, "connector_id", claims.ConnectorID)
		h.fail(w, endpoint, http.StatusInternalServerError, "internal_error", "Failed to create the user operation")
		return
	}

	h.logger.Info("User operation created", "type", op.Type, "operation_name", name,
		"user_id", claims.UserID, "connector_id", claims.ConnectorID, "remote_addr", r.RemoteAddr)

	status, done := h.waitForOperation(r.Context(), name)
	switch {
	case !done:
		h.count(endpoint, http.StatusAccepted)
		h.writeJSON(w, http.StatusAccepted, OperationResponse{Status: statusPending, OperationName: name})
	case status.Phase == k8s.PhaseSucceeded:
		h.count(endpoint, http.StatusOK)
		h.writeJSON(w, http.StatusOK, OperationResponse{Status: statusDone, OperationName: name, Message: status.Message})
	case status.Message == unsupportedTypeMessage+op.Type:
		h.logger.Warn("User operation refused by a controller that does not know its type yet; the controller is being updated",
			"type", op.Type, "operation_name", name)
		h.count(endpoint, http.StatusServiceUnavailable)
		w.Header().Set("Retry-After", retryAfterControllerUpdate)
		h.writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:         "temporarily_unavailable",
			Message:       "The user-authn controller is being updated, try again later",
			OperationName: name,
		})
	default:
		h.logger.Warn("User operation failed", "type", op.Type, "operation_name", name, "message", status.Message)
		h.count(endpoint, http.StatusUnprocessableEntity)
		h.writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{
			Error:         "operation_failed",
			Message:       status.Message,
			OperationName: name,
		})
	}
}

// waitForOperation polls the operation until the controller sets its phase. It reports false when
// the wait ran out or the client went away first.
func (h *Handler) waitForOperation(ctx context.Context, name string) (k8s.OperationStatus, bool) {
	ctx, cancel := context.WithTimeout(ctx, h.waitTimeout)
	defer cancel()

	ticker := time.NewTicker(h.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return k8s.OperationStatus{}, false
		case <-ticker.C:
		}

		status, err := h.k8sClient.UserOperationStatus(ctx, name)
		if err != nil {
			// A failed read ends nothing: the operation runs regardless, and the next poll or the
			// 202 at the end tells the client how it went.
			if ctx.Err() == nil {
				h.logger.Warn("Failed to read user operation status", "error", err, "operation_name", name)
			}
			continue
		}
		if status.Phase == k8s.PhaseSucceeded || status.Phase == k8s.PhaseFailed {
			return status, true
		}
	}
}

// readSelfRequest accepts an empty body or an empty JSON object.
func readSelfRequest(w http.ResponseWriter, r *http.Request) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize))
	decoder.DisallowUnknownFields()

	var req selfRequest
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	if decoder.More() {
		return errors.New("unexpected data after the JSON object")
	}
	return nil
}

// retryAfterSeconds renders a delay as the whole seconds of a Retry-After header, rounded up.
func retryAfterSeconds(delay time.Duration) string {
	return strconv.Itoa(int(math.Ceil(delay.Seconds())))
}

func (h *Handler) count(endpoint string, status int) {
	h.reqCounter.WithLabelValues(endpoint, strconv.Itoa(status)).Inc()
}

func (h *Handler) fail(w http.ResponseWriter, endpoint string, status int, errCode, message string) {
	h.count(endpoint, status)
	h.writeError(w, status, errCode, message)
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("Failed to encode response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, errCode, message string) {
	h.writeJSON(w, status, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}
