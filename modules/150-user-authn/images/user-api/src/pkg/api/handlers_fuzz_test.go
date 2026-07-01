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

	// минимальный seed corpus
	f.Add("Bearer validtoken", validBcryptHash)
	f.Add("", validBcryptHash)
	f.Add("Bearer invalidtoken", "")
	f.Add("Basic abc", "plaintext")
	f.Add("Bearer validtoken", "$2y$10$invalidhash")

	f.Fuzz(func(t *testing.T, authHeader string, passwordHash string) {

		verifier := &mockVerifier{
			claims: &auth.Claims{
				Username:    "fuzz-user",
				Email:       "test@example.com",
				ConnectorID: "local",
			},
		}

		k8sClient := &mockK8sClient{
			isLocal:       true,
			operationName: "self-password-reset-fuzz",
		}

		// делаем только одну контролируемую вариацию ошибки
		if authHeader == "Bearer invalidtoken" {
			verifier.err = auth.ErrTokenValidation
		}

		h := newTestHandler(verifier, k8sClient)

		body := PasswordResetRequest{
			NewPasswordHash: passwordHash,
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

		// базовая проверка: HTTP всегда валидный
		if w.Code < 100 || w.Code > 599 {
			t.Fatalf("invalid HTTP status: %d", w.Code)
		}

		// проверка успешного ответа
		if w.Code == http.StatusAccepted {
			var resp PasswordResetResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("bad JSON: %v body=%q", err, w.Body.String())
			}

			if resp.OperationName == "" {
				t.Fatalf("empty operation name")
			}
		}

		// проверка error response
		if w.Code >= http.StatusBadRequest {
			var errResp ErrorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("bad error JSON: %v body=%q", err, w.Body.String())
			}

			if errResp.Error == "" {
				t.Fatalf("empty error code")
			}
		}
	})
}
