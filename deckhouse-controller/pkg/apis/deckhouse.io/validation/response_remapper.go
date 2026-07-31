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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// withInvalidReason makes `kubectl edit` reopen the editor on validation
// failure: kubewebhook hard-codes HTTP 400 in the AdmissionResponse, but
// kubectl reopens the editor only on Reason=Invalid / HTTP 422. We buffer the
// upstream response, and if it denies the request, rewrite Result to 422 +
// Invalid before sending it to the client.
func withInvalidReason(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)

		body := rec.Body.Bytes()

		var review admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &review); err == nil && review.Response != nil && !review.Response.Allowed {
			msg := ""
			var details *metav1.StatusDetails

			if review.Response.Result != nil {
				msg = review.Response.Result.Message
				if causes := messageToCauses(msg); len(causes) > 0 {
					details = &metav1.StatusDetails{Causes: causes}
				}
			}

			review.Response.Result = &metav1.Status{
				Status:  metav1.StatusFailure,
				Reason:  metav1.StatusReasonInvalid,
				Code:    http.StatusUnprocessableEntity,
				Message: msg,
				Details: details,
			}
			if patched, err := json.Marshal(review); err == nil {
				body = patched
			}
		}

		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(body)
	})
}

// messageToCauses splits a composite validation message into one cause per
// line so that `kubectl edit` renders each reason on its own bullet instead of
// a single blob. Validators often join several reasons with "\n- " (see
// validate_deckhouse_release.go), so we strip leading bullet markers and drop
// blank lines. A single-line message yields a single cause; an empty or
// blank-only message yields no causes.
func messageToCauses(msg string) []metav1.StatusCause {
	var causes []metav1.StatusCause
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		causes = append(causes, metav1.StatusCause{Message: line})
	}
	return causes
}
