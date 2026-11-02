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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{}, State: s, Gate: serving, Metrics: NewMetrics(prometheus.NewRegistry())}

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
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{},
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
	h := &ObjectHandler{Family: FamilyVM, Evaluator: evaluator.VirtualMachine{},
		State: s, Gate: serving, Metrics: NewMetrics(prometheus.NewRegistry())}
	review := new(admissionv1.AdmissionReview)
	require.NoError(t, json.Unmarshal(vmReview(t, admissionv1.Update, "one", 2, 2), review))
	review.Request.OldObject.Raw = bytes.Replace(review.Request.OldObject.Raw, []byte(`"1Gi"`), []byte(`"a lot"`), 1)
	body, err := json.Marshal(review)
	require.NoError(t, err)
	response := callObject(t, h, body)
	assert.True(t, response.Allowed, response.Result)
}
