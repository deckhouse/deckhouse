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
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"

	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

type Handler struct {
	verifier   auth.Verifier
	k8sClient  k8s.Client
	logger     *slog.Logger
	ready      atomic.Bool
	registry   *prometheus.Registry
	reqCounter *prometheus.CounterVec
}

type PasswordResetRequest struct {
	NewPasswordHash string `json:"newPasswordHash"`
}

type PasswordResetResponse struct {
	Status        string `json:"status"`
	OperationName string `json:"operationName,omitempty"`
	Message       string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

const maxRequestBodySize = 1 << 20 // 1 MB

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
		verifier:   verifier,
		k8sClient:  k8sClient,
		logger:     logger,
		registry:   registry,
		reqCounter: reqCounter,
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

func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	token, err := h.verifier.ExtractToken(r)
	if err != nil {
		h.logger.Warn("Failed to extract token", "error", err, "remote_addr", r.RemoteAddr)
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "401").Inc()
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid authorization header")
		return
	}

	claims, err := h.verifier.Verify(ctx, token)
	if err != nil {
		h.logger.Warn("Failed to verify token", "error", err, "remote_addr", r.RemoteAddr)
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "401").Inc()
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid token")
		return
	}

	h.logger.Info("Password reset request", "username", claims.Username, "remote_addr", r.RemoteAddr)

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req PasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warn("Failed to decode request body", "error", err, "username", claims.Username)
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "400").Inc()
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body")
		return
	}

	if req.NewPasswordHash == "" {
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "400").Inc()
		h.writeError(w, http.StatusBadRequest, "invalid_request", "newPasswordHash is required")
		return
	}

	if !strings.HasPrefix(req.NewPasswordHash, "$2") {
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "400").Inc()
		h.writeError(w, http.StatusBadRequest, "invalid_request", "newPasswordHash must be a bcrypt hash (starting with $2)")
		return
	}

	if _, err := bcrypt.Cost([]byte(req.NewPasswordHash)); err != nil {
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "400").Inc()
		h.writeError(w, http.StatusBadRequest, "invalid_request", "newPasswordHash must be a valid bcrypt hash")
		return
	}

	isLocal, err := h.k8sClient.IsLocalUser(ctx, claims.Username)
	if err != nil {
		h.logger.Error("Failed to check if user is local", "error", err, "username", claims.Username)
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "500").Inc()
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to verify user")
		return
	}

	if !isLocal {
		h.logger.Warn("User is not a local user", "username", claims.Username)
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "403").Inc()
		h.writeError(w, http.StatusForbidden, "forbidden", "Password reset is only available for local users")
		return
	}

	operationName, err := h.k8sClient.CreatePasswordResetOperation(ctx, claims.Username, req.NewPasswordHash)
	if err != nil {
		h.logger.Error("Failed to create password reset operation", "error", err, "username", claims.Username)
		if errors.Is(err, k8s.ErrOperationFailed) {
			h.reqCounter.WithLabelValues("/api/v1/password/reset", "500").Inc()
			h.writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create password reset operation")
			return
		}
		h.reqCounter.WithLabelValues("/api/v1/password/reset", "500").Inc()
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Unexpected error")
		return
	}

	h.logger.Info("Password reset operation created", "username", claims.Username, "operation_name", operationName)
	h.reqCounter.WithLabelValues("/api/v1/password/reset", "202").Inc()

	h.writeJSON(w, http.StatusAccepted, PasswordResetResponse{
		Status:        "accepted",
		OperationName: operationName,
		Message:       "Password reset operation created successfully",
	})
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
