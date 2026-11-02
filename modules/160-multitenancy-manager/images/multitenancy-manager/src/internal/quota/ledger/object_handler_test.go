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
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"controller/internal/quota/evaluator"
)

func vmReview(t *testing.T, operation admissionv1.Operation, name string, oldCores, newCores int64) []byte {
	t.Helper()
	object := func(cores int64) runtime.RawExtension {
		raw, err := json.Marshal(map[string]any{
			"apiVersion": "virtualization.deckhouse.io/v1alpha2", "kind": "VirtualMachine",
			"metadata": map[string]any{"name": name, "namespace": "a", "resourceVersion": "10"},
			"spec":     map[string]any{"cpu": map[string]any{"cores": cores, "coreFraction": "100%"}, "memory": map[string]any{"size": "1Gi"}},
		})
		require.NoError(t, err)
		return runtime.RawExtension{Raw: raw}
	}
	request := &admissionv1.AdmissionRequest{UID: "uid-vm", Namespace: "a", Name: name, Operation: operation, Object: object(newCores)}
	if operation == admissionv1.Update {
		request.OldObject = object(oldCores)
	}
	body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Request: request})
	require.NoError(t, err)
	return body
}

func callObject(t *testing.T, h http.Handler, body []byte) *admissionv1.AdmissionResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/validate/quota-vm", bytes.NewReader(body)))
	require.Equal(t, http.StatusOK, rec.Code)
	out := new(admissionv1.AdmissionReview)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
	return out.Response
}

func TestObjectHandler(t *testing.T) {
	s := vmState()
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{}, NewObject: func() runtime.Object { return new(unstructured.Unstructured) },
		State: s, Gate: serving, Metrics: NewMetrics(prometheus.NewRegistry())}

	response := callObject(t, h, vmReview(t, admissionv1.Create, "one", 0, 6))
	assert.True(t, response.Allowed)
	assert.Equal(t, 1, s.ObjectPendingCount())

	response = callObject(t, h, vmReview(t, admissionv1.Update, "one", 6, 10))
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusForbidden), response.Result.Code)
	assert.Contains(t, response.Result.Message, "exceeded quota: d8-project-quota, project p:")

	response = callObject(t, h, vmReview(t, admissionv1.Update, "one", 6, 2))
	assert.True(t, response.Allowed, "a machine made smaller passes")

	h.Gate = func() (string, string) { return ResultNotLeader, "not the leader" }
	response = callObject(t, h, vmReview(t, admissionv1.Create, "two", 0, 1))
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code)
	response = callObject(t, h, vmReview(t, admissionv1.Update, "one", 6, 2))
	assert.True(t, response.Allowed, "a machine made smaller passes while the ledger does not answer")
	response = callObject(t, h, vmReview(t, admissionv1.Update, "one", 2, 3))
	assert.False(t, response.Allowed, "a machine made bigger waits for the ledger")

	h.Suspended = func() bool { return true }
	assert.True(t, callObject(t, h, vmReview(t, admissionv1.Create, "three", 0, 100)).Allowed, "a suspended ledger lets every write through")
}

// The webhooks of the families share the limit of the calls of one source in progress with the
// webhook of the quotas: a caller that holds its places, an API server or a forged caller alike, is
// answered overload instead of having more of its requests recorded.
func TestObjectHandlerSharesTheLimitOfASource(t *testing.T) {
	s := vmState()
	sources := &Handler{}
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{}, NewObject: func() runtime.Object { return new(unstructured.Unstructured) },
		State: s, Gate: serving, Metrics: NewMetrics(prometheus.NewRegistry()), Sources: sources}
	source := sourceOf(httptest.NewRequest(http.MethodPost, "/", nil).RemoteAddr)
	for range perSourceLimit {
		require.True(t, sources.enter(source))
	}
	response := callObject(t, h, vmReview(t, admissionv1.Create, "forged", 0, 8))
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code)
	assert.Equal(t, types.UID("uid-vm"), response.UID, "the refusal of an overload echoes the request")
	assert.Zero(t, s.ObjectPendingCount(), "nothing recorded")

	sources.leave(source)
	assert.True(t, callObject(t, h, vmReview(t, admissionv1.Create, "one", 0, 1)).Allowed)
	assert.Equal(t, 1, len(sources.inflight), "the place is given back")
}

// An old object that does not evaluate does not lock the object: the write that fixes it is judged
// by what it puts.
func TestObjectHandlerTakesAnUnreadableOldObjectAsNothing(t *testing.T) {
	s := vmState()
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{}, NewObject: func() runtime.Object { return new(unstructured.Unstructured) },
		State: s, Gate: serving, Metrics: NewMetrics(prometheus.NewRegistry())}
	review := new(admissionv1.AdmissionReview)
	require.NoError(t, json.Unmarshal(vmReview(t, admissionv1.Update, "one", 2, 2), review))
	review.Request.OldObject.Raw = bytes.Replace(review.Request.OldObject.Raw, []byte(`"1Gi"`), []byte(`"a lot"`), 1)
	body, err := json.Marshal(review)
	require.NoError(t, err)
	response := callObject(t, h, body)
	assert.True(t, response.Allowed, response.Result)
}

func lbReview(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := json.Marshal(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "a", ResourceVersion: "10"},
		Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer}})
	require.NoError(t, err)
	body, err := json.Marshal(admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Request: &admissionv1.AdmissionRequest{UID: "uid-lb", Namespace: "a", Name: name, Operation: admissionv1.Create, Object: runtime.RawExtension{Raw: raw}}})
	require.NoError(t, err)
	return body
}

// Every answer of the webhooks of the families echoes the uid of the request, a refusal too: the API
// server takes an answer without it for an invalid one and fails the request with an internal error.
func TestObjectHandlersEchoTheUIDOnEveryAnswer(t *testing.T) {
	families := []struct {
		name   string
		uid    types.UID
		body   func(name string) []byte
		object func() runtime.Object
		eval   evaluator.Evaluator
		state  func(limit string) *State
	}{
		{name: "vm", uid: "uid-vm", body: func(name string) []byte { return vmReview(t, admissionv1.Create, name, 0, 4) },
			object: func() runtime.Object { return new(unstructured.Unstructured) }, eval: evaluator.VirtualMachine{},
			state: func(limit string) *State {
				s := NewState()
				s.SetProjectQuota(pq("p", rl(cores, limit)))
				s.SetNamespace(ns("a", "p"))
				return s
			}},
		{name: "lb", uid: "uid-lb", body: func(name string) []byte { return lbReview(t, name) },
			object: func() runtime.Object { return new(corev1.Service) }, eval: evaluator.LoadBalancer{},
			state: func(limit string) *State {
				s := NewState()
				s.SetProjectQuota(pq("p", rl(lbAddresses, limit)))
				s.SetNamespace(ns("a", "p"))
				return s
			}},
	}
	for _, family := range families {
		cases := []struct {
			result string
			limit  string
			gate   GateFunc
			full   bool
			susp   bool
		}{
			{result: ResultAllowed, limit: "100", gate: serving},
			{result: ResultExceeded, limit: "0", gate: serving},
			{result: ResultNotLeader, limit: "100", gate: func() (string, string) { return ResultNotLeader, "test" }},
			{result: ResultOverload, limit: "100", gate: serving, full: true},
			{result: ResultSuspended, limit: "0", gate: serving, susp: true},
		}
		for _, tc := range cases {
			t.Run(family.name+"/"+tc.result, func(t *testing.T) {
				sources := &Handler{}
				if tc.full {
					for range perSourceLimit {
						sources.enter(sourceOf(httptest.NewRequest(http.MethodPost, "/", nil).RemoteAddr))
					}
				}
				h := &ObjectHandler{Family: family.name, Evaluator: family.eval, NewObject: family.object, State: family.state(tc.limit),
					Gate: tc.gate, Metrics: NewMetrics(prometheus.NewRegistry()), Sources: sources, Suspended: func() bool { return tc.susp }}
				response := callObject(t, h, family.body("one"))
				assert.Equal(t, family.uid, response.UID)
				assert.InDelta(t, 1, testutil.ToFloat64(h.Metrics.Requests.WithLabelValues(tc.result)), 0, "the answer is the one of the case")
			})
		}
	}
}
