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

package webhooks

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// webhookDecisionTimeout hard-bounds how long a single admission decision may take. The handlers read
// from the API on the hot path; without an internal bound a slow read would hold the request until the
// webhook's 10s deadline and, with failurePolicy: Fail, let addon-operator pile up retries into a
// multi-minute queue lock. A short, terminal bound guarantees the webhook always answers quickly — it
// can never become the thing that locks a queue.
const webhookDecisionTimeout = 3 * time.Second

// decodeReview reads and decodes an AdmissionReview from a JSON request body.
func decodeReview(r *http.Request, review *admissionv1.AdmissionReview) error {
	if ct := r.Header.Get("Content-Type"); ct != "application/json" {
		return errors.New("unexpected Content-Type: " + ct)
	}
	return json.NewDecoder(r.Body).Decode(review)
}

// writeReview writes an AdmissionReview as the JSON response.
func writeReview(w http.ResponseWriter, review *admissionv1.AdmissionReview) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(review)
}

// allowedResponse builds an allow response.
func allowedResponse(uid types.UID) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{UID: uid, Allowed: true}
}

// deniedResponse builds a deny response with a forbidden status and message.
func deniedResponse(uid types.UID, message string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		UID:     uid,
		Allowed: false,
		Result: &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: message,
			Reason:  metav1.StatusReasonForbidden,
			Code:    http.StatusForbidden,
		},
	}
}

// jsonPatchOperation is a single RFC6902 JSON Patch operation.
type jsonPatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}
