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

package validation

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWithInvalidReason(t *testing.T) {
	deniedBody := mustMarshalReview(t, admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Response: &admissionv1.AdmissionResponse{
			UID:     "uid-denied",
			Allowed: false,
			Result: &metav1.Status{
				Message: "spec.version=999 is unsupported",
				Code:    http.StatusBadRequest,
			},
		},
	})

	allowedBody := mustMarshalReview(t, admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Response: &admissionv1.AdmissionResponse{
			UID:      "uid-allowed",
			Allowed:  true,
			Warnings: []string{"deprecated field"},
		},
	})

	tests := []struct {
		name      string
		giveBody  []byte
		giveCode  int
		wantCode  int
		wantCheck func(t *testing.T, body []byte)
	}{
		{
			name:     "denied rewritten to 422 Invalid with original message preserved",
			giveBody: deniedBody,
			giveCode: http.StatusOK,
			wantCode: http.StatusOK,
			wantCheck: func(t *testing.T, body []byte) {
				var got admissionv1.AdmissionReview
				require.NoError(t, json.Unmarshal(body, &got))
				require.NotNil(t, got.Response)
				require.NotNil(t, got.Response.Result)
				assert.False(t, got.Response.Allowed)
				assert.Equal(t, metav1.StatusReasonInvalid, got.Response.Result.Reason)
				assert.Equal(t, int32(http.StatusUnprocessableEntity), got.Response.Result.Code)
				assert.Equal(t, metav1.StatusFailure, got.Response.Result.Status)
				assert.Equal(t, "spec.version=999 is unsupported", got.Response.Result.Message)
			},
		},
		{
			name:     "allowed response passes through unchanged",
			giveBody: allowedBody,
			giveCode: http.StatusOK,
			wantCode: http.StatusOK,
			wantCheck: func(t *testing.T, body []byte) {
				var got admissionv1.AdmissionReview
				require.NoError(t, json.Unmarshal(body, &got))
				require.NotNil(t, got.Response)
				assert.True(t, got.Response.Allowed)
				assert.Nil(t, got.Response.Result)
				assert.Equal(t, []string{"deprecated field"}, got.Response.Warnings)
			},
		},
		{
			name:     "non-AdmissionReview body passes through unchanged",
			giveBody: []byte("internal server error"),
			giveCode: http.StatusInternalServerError,
			wantCode: http.StatusInternalServerError,
			wantCheck: func(t *testing.T, body []byte) {
				assert.Equal(t, "internal server error", string(body))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.giveCode)
				_, err := w.Write(tt.giveBody)
				require.NoError(t, err)
			})

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader([]byte("{}")))

			withInvalidReason(upstream).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantCode, rec.Code)
			tt.wantCheck(t, rec.Body.Bytes())
		})
	}
}

func mustMarshalReview(t *testing.T, review admissionv1.AdmissionReview) []byte {
	t.Helper()
	out, err := json.Marshal(review)
	require.NoError(t, err)
	return out
}
