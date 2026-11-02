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

package ledger

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"controller/internal/quota/evaluator"
)

// ObjectHandler serves the webhook of a family of objects: the creates and updates of its objects in
// the namespaces of the projects whose quota has keys of the family.
type ObjectHandler struct {
	Family    string
	Evaluator evaluator.Evaluator
	State     *State
	Gate      GateFunc
	Metrics   *Metrics
	// Suspended reports the quota-ledger-suspend ConfigMap. Optional.
	Suspended func() bool
	// Sources limits the calls of one source in progress on this webhook, apart from the webhook of the
	// quotas, whose limit keeps workers of the quota plugin of an API server free. Optional.
	Sources *Handler
	// Now is time.Now unless a test sets it.
	Now func() time.Time
}

func (h *ObjectHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *ObjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	responder := &Handler{Metrics: h.Metrics}
	// the place of the source is taken before the body is read; a write that waits for room takes one
	// of the places of its source that may wait instead
	var pause func() bool
	if h.Sources != nil {
		source := sourceOf(r.RemoteAddr)
		if !h.Sources.enter(source) {
			responder.refuseOverload(w, r)
			return
		}
		held := &place{h: h.Sources, source: source}
		defer held.release()
		pause = held.wait
	}
	review := new(admissionv1.AdmissionReview)
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReviewBytes))
	if err != nil || json.Unmarshal(body, review) != nil || review.Request == nil {
		responder.count(ResultBadRequest)
		http.Error(w, "the body is not an AdmissionReview", http.StatusBadRequest)
		return
	}
	if h.Suspended != nil && h.Suspended() {
		responder.respond(w, review, allow(ResultSuspended), false, start)
		return
	}
	in, err := h.inputOf(review.Request)
	if err != nil {
		responder.respond(w, review, Decision{Code: http.StatusBadRequest, Result: ResultBadRequest, Message: err.Error()}, false, start)
		return
	}
	// the objects of the request are read; a write that waits keeps only the uid of the request
	review.Request.Object.Raw, review.Request.OldObject.Raw = nil, nil
	// a write that raises no key of the object it replaces cannot take the project over its quota, so it
	// does not wait for the ledger: stopping a machine or making it smaller frees the quota even while no
	// replica answers
	if raisesNothing(in) {
		responder.respond(w, review, allow(ResultNotRaised), false, start)
		return
	}
	if result, cause := h.Gate(); result != "" {
		responder.respond(w, review, unavailable(result, cause, false), true, start)
		return
	}
	decide := func() Decision {
		return decideWithin(func(claim func() bool) Decision { return h.State.DecideObject(in, h.now(), claim) })
	}
	responder.respond(w, review, decideWithRoom(r.Context(), h.State, in.Namespace, h.Gate, h.Metrics, pause, decide), false, start)
}

// inputOf reads the write from the request: the usage of the object and of the object it replaces.
func (h *ObjectHandler) inputOf(req *admissionv1.AdmissionRequest) (ObjectInput, error) {
	in := ObjectInput{Family: h.Family, Namespace: req.Namespace, Name: req.Name, DryRun: req.DryRun != nil && *req.DryRun}
	written := new(unstructured.Unstructured)
	if err := json.Unmarshal(req.Object.Raw, &written.Object); err != nil {
		return in, fmt.Errorf("read the object: %w", err)
	}
	if in.Name == "" {
		in.Name = written.GetName()
	}
	var err error
	if in.NewUsed, err = h.Evaluator.Usage(written); err != nil {
		return in, fmt.Errorf("evaluate the object for the project quota: %w", err)
	}
	if req.Operation != admissionv1.Update || len(req.OldObject.Raw) == 0 {
		in.OldUsed = corev1.ResourceList{}
		return in, nil
	}
	old := new(unstructured.Unstructured)
	if err := json.Unmarshal(req.OldObject.Raw, &old.Object); err != nil {
		return in, fmt.Errorf("read the old object: %w", err)
	}
	in.OldRV = ParseResourceVersion(old.GetResourceVersion())
	// An old object that does not evaluate is taken as using nothing, so that a write that fixes it
	// is judged by what it puts instead of refused.
	if in.OldUsed, err = h.Evaluator.Usage(old); err != nil {
		in.OldUsed = corev1.ResourceList{}
	}
	return in, nil
}

// raisesNothing reports whether the write updates an object without raising any key of its usage.
func raisesNothing(in ObjectInput) bool {
	if in.OldRV == 0 {
		return false
	}
	for key, value := range in.NewUsed {
		if old := in.OldUsed[key]; value.Cmp(old) > 0 {
			return false
		}
	}
	return true
}
