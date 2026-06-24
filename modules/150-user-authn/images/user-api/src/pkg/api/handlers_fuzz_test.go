/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"user-api/pkg/auth"
)

func FuzzResetPassword(f *testing.F) {
	validBcryptHash := "$2a$10$fyCj7unkBxyxSrh6jULkTOwvUabNzdd81am02CT9tWTlb.y0Eptri"

	f.Add("Bearer validtoken", validBcryptHash, "testuser", "local", true)
	f.Add("", validBcryptHash, "testuser", "local", true)
	f.Add("Basic abc", validBcryptHash, "testuser", "local", true)
	f.Add("Bearer invalidtoken", validBcryptHash, "testuser", "local", true)
	f.Add("Bearer validtoken", "", "testuser", "local", true)
	f.Add("Bearer validtoken", "plaintext", "testuser", "local", true)
	f.Add("Bearer validtoken", "$2y$10$invalidhash", "testuser", "local", true)
	f.Add("Bearer validtoken", validBcryptHash, "testuser", "github", true)
	f.Add("Bearer validtoken", validBcryptHash, "externaluser", "local", false)

	f.Fuzz(func(t *testing.T, authHeader string, newPasswordHash string, username string, connectorID string, isLocal bool) {
		verifier := &mockVerifier{
			claims: &auth.Claims{
				Username:    username,
				Email:       "test@example.com",
				ConnectorID: connectorID,
			},
		}

		k8sClient := &mockK8sClient{
			isLocal:       isLocal,
			operationName: "self-password-reset-fuzz",
		}

		if authHeader == "Bearer invalidtoken" {
			verifier.err = auth.ErrTokenValidation
		}

		h := newTestHandler(verifier, k8sClient)

		body := PasswordResetRequest{
			NewPasswordHash: newPasswordHash,
		}

		bodyBytes, err := json.Marshal(body)
		if err != nil {
			t.Skip()
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/password/reset", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		w := httptest.NewRecorder()
		h.ResetPassword(w, req)

		if w.Code < 100 || w.Code > 599 {
			t.Fatalf("invalid HTTP status: %d", w.Code)
		}

		if w.Code == http.StatusAccepted {
			var resp PasswordResetResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("accepted response is not valid JSON: %v, body=%q", err, w.Body.String())
			}

			if resp.Status == "" {
				t.Fatalf("accepted response has empty status, body=%q", w.Body.String())
			}

			if resp.OperationName == "" {
				t.Fatalf("accepted response has empty operationName, body=%q", w.Body.String())
			}
		}

		if w.Code >= http.StatusBadRequest {
			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("error response is not valid JSON: %v, status=%d, body=%q", err, w.Code, w.Body.String())
			}

			if errResp.Error == "" {
				t.Fatalf("error response has empty error code, status=%d, body=%q", w.Code, w.Body.String())
			}
		}
	})
}
