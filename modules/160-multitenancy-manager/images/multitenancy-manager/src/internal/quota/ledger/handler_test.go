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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func serving() (string, string) { return "", "" }

func newHandler(s *State, gate GateFunc) *Handler {
	return &Handler{State: s, Gate: gate, Metrics: NewMetrics(prometheus.NewRegistry())}
}

func reviewBody(t *testing.T, namespace string, rv uint64, oldUsed, newUsed corev1.ResourceList) []byte {
	t.Helper()
	raw := func(used corev1.ResourceList, rv uint64) runtime.RawExtension {
		object := rq(namespace, rv, used)
		data, err := json.Marshal(object)
		require.NoError(t, err)
		return runtime.RawExtension{Raw: data}
	}
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("uid-1"), Namespace: namespace, Name: "d8-project-quota", Operation: admissionv1.Update, SubResource: "status",
			UserInfo: authenticationv1.UserInfo{Username: APIServerUser},
			Object:   raw(newUsed, rv), OldObject: raw(oldUsed, rv),
		},
	}
	body, err := json.Marshal(review)
	require.NoError(t, err)
	return body
}

func call(t *testing.T, h *Handler, body []byte) (*httptest.ResponseRecorder, *admissionv1.AdmissionResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/validate/quota-ledger", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.1:43210"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return rec, nil
	}
	out := new(admissionv1.AdmissionReview)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
	return rec, out.Response
}

func TestHandlerAnswers(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5"))
	h := newHandler(s, serving)

	_, response := call(t, h, reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5")))
	require.NotNil(t, response)
	assert.True(t, response.Allowed)
	assert.Equal(t, types.UID("uid-1"), response.UID)
	assert.Equal(t, 1, s.PendingCount())

	_, response = call(t, h, reviewBody(t, "b", 20, rl("pods", "5"), rl("pods", "6")))
	require.NotNil(t, response)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusForbidden), response.Result.Code)
	assert.Contains(t, response.Result.Message, "exceeded quota: d8-project-quota, project p:")
	assert.InDelta(t, 1, testutil.ToFloat64(h.Metrics.Requests.WithLabelValues(ResultExceeded)), 0)
}

// A replica that does not answer closes the connection, so the API server reaches the pod the Service
// points to now.
func TestHandlerClosesTheConnectionWhenItDoesNotAnswer(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5"))
	for _, result := range []string{ResultNotLeader, ResultFenced, ResultSyncing} {
		h := newHandler(s, func() (string, string) { return result, "test" })
		rec, response := call(t, h, reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5")))
		require.NotNil(t, response, result)
		assert.False(t, response.Allowed, result)
		assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code, result)
		assert.Equal(t, "project quota ledger unavailable (test), retry", response.Result.Message)
		assert.Equal(t, "close", rec.Header().Get("Connection"), result)
	}
	assert.Zero(t, s.PendingCount())
}

func TestHandlerLimitsCallsPerSource(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5"))
	h := newHandler(s, serving)
	for range perSourceLimit {
		require.True(t, h.enter("10.0.0.1"))
	}
	_, response := call(t, h, reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5")))
	require.NotNil(t, response)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code)
	assert.Contains(t, response.Result.Message, "overload")
	assert.True(t, h.enter("10.0.0.2"), "another API server is not limited by the first")
}

// Every answer echoes the uid of the request, a refusal too: the API server takes an answer without
// it for an invalid one and fails the request with an internal error instead of the refusal.
func TestHandlerEchoesTheUIDOnEveryAnswer(t *testing.T) {
	body := reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5"))
	cases := []struct {
		result string
		h      func() *Handler
		body   []byte
	}{
		{result: ResultAllowed, h: func() *Handler { return newHandler(state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5")), serving) }, body: body},
		{result: ResultExceeded, h: func() *Handler { return newHandler(state(rl("pods", "9"), rl("pods", "4"), rl("pods", "5")), serving) }, body: body},
		{result: ResultUnknownNamespace, h: func() *Handler { return newHandler(NewState(), serving) }, body: body},
		{result: ResultNotLeader, h: func() *Handler {
			return newHandler(NewState(), func() (string, string) { return ResultNotLeader, "test" })
		}, body: body},
		{result: ResultOverload, h: func() *Handler {
			h := newHandler(state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5")), serving)
			for range perSourceLimit {
				h.enter("10.0.0.1")
			}
			return h
		}, body: body},
		{result: ResultInconsistent, h: func() *Handler { return newHandler(state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5")), serving) },
			body: reviewBody(t, "a", 10, rl("pods", "1"), rl("pods", "2"))},
	}
	for _, tc := range cases {
		t.Run(tc.result, func(t *testing.T) {
			h := tc.h()
			_, response := call(t, h, tc.body)
			require.NotNil(t, response)
			assert.Equal(t, types.UID("uid-1"), response.UID)
			assert.InDelta(t, 1, testutil.ToFloat64(h.Metrics.Requests.WithLabelValues(tc.result)), 0, "the answer is the one of the case")
		})
	}
}

// confirmLoop plays the informer: it shows every approved write as written a few milliseconds later.
func confirmLoop(t *testing.T, s *State, stop <-chan struct{}) {
	t.Helper()
	for {
		select {
		case <-stop:
			return
		case <-time.After(5 * time.Millisecond):
		}
		s.mu.Lock()
		var written []*corev1.ResourceQuota
		for namespace, q := range s.quotas {
			for _, p := range q.pending {
				written = append(written, rq(namespace, p.rvOld+1, p.used))
			}
		}
		s.mu.Unlock()
		for _, quota := range written {
			s.SetQuota(quota)
		}
	}
}

// With a small limit the unconfirmed approvals of a project leave room for one write at a time.
// Parallel writes wait for the approvals before them to be confirmed instead of being answered busy:
// the ones that fit are admitted and the others refused over the quota.
func TestHandlerWaitsForRoom(t *testing.T) {
	s := NewState()
	s.SetProjectQuota(pq("p", rl("pods", "3")))
	namespaces := []string{"n0", "n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "n9"}
	for _, name := range namespaces {
		s.SetNamespace(ns(name, "p"))
		s.SetQuota(rq(name, 10, rl("pods", "0")))
	}
	h := newHandler(s, serving)
	stop := make(chan struct{})
	defer close(stop)
	go confirmLoop(t, s, stop)

	// perSourceWaitLimit calls per API server, so that all of them may wait for room: a third waiting
	// write of one API server is answered busy (TestWaitingWritesLeaveTheirSource)
	results := make(chan *admissionv1.AdmissionResponse, len(namespaces))
	var wg sync.WaitGroup
	for i, name := range namespaces {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/validate/quota-ledger", bytes.NewReader(reviewBody(t, name, 10, rl("pods", "0"), rl("pods", "1"))))
			req.RemoteAddr = fmt.Sprintf("10.0.0.%d:43210", i/perSourceWaitLimit)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			out := new(admissionv1.AdmissionReview)
			if json.Unmarshal(rec.Body.Bytes(), out) == nil {
				results <- out.Response
			}
		}()
	}
	wg.Wait()
	close(results)
	admitted, exceeded := 0, 0
	for response := range results {
		require.NotNil(t, response)
		switch {
		case response.Allowed:
			admitted++
		case response.Result.Code == http.StatusForbidden:
			exceeded++
		}
	}
	assert.Equal(t, 3, admitted)
	assert.Equal(t, 7, exceeded, "the rest are refused over the quota, not answered busy")
}

// A write that finds no room within the wait is answered busy, with the uid of its request; one whose
// replica stops answering during the wait is answered so.
func TestHandlerWaitForRoomEnds(t *testing.T) {
	s := state(rl("pods", "3"), rl("pods", "0"), rl("pods", "0"))
	h := newHandler(s, serving)
	_, first := call(t, h, reviewBody(t, "a", 10, rl("pods", "0"), rl("pods", "1")))
	require.True(t, first.Allowed)

	started := time.Now()
	_, response := call(t, h, reviewBody(t, "b", 20, rl("pods", "0"), rl("pods", "1")))
	require.NotNil(t, response)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code)
	assert.Contains(t, response.Result.Message, "too many approvals")
	assert.Equal(t, types.UID("uid-1"), response.UID)
	assert.GreaterOrEqual(t, time.Since(started), inflightWait)
	assert.InDelta(t, 1, testutil.ToFloat64(h.Metrics.InflightWaits.WithLabelValues("timeout")), 0)

	var fenced atomic.Bool
	h.Gate = func() (string, string) {
		if fenced.Load() {
			return ResultFenced, "fenced"
		}
		return "", ""
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		fenced.Store(true)
		s.SetQuota(rq("a", 11, rl("pods", "1")))
	}()
	_, response = call(t, h, reviewBody(t, "b", 20, rl("pods", "0"), rl("pods", "1")))
	require.NotNil(t, response)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "fenced")
	assert.Zero(t, s.PendingCount(), "nothing approved after the fence passed")
}

// countingReader counts what is read of a body that is far larger than a review.
type countingReader struct {
	prefix []byte
	read   int
}

func (r *countingReader) Read(p []byte) (int, error) {
	for i := range p {
		if r.read < len(r.prefix) {
			p[i] = r.prefix[r.read]
		} else {
			p[i] = ' '
		}
		r.read++
	}
	return len(p), nil
}

// A source with no place left is refused before its body is read: only the first bytes, for the uid
// of the request, so that the calls of one source never hold more than a few bodies in memory.
func TestHandlerRefusesAnOverloadBeforeReadingTheBody(t *testing.T) {
	h := newHandler(state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5")), serving)
	for range perSourceLimit {
		require.True(t, h.enter("10.0.0.1"))
	}
	body := &countingReader{prefix: reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5"))}
	req := httptest.NewRequest(http.MethodPost, "/validate/quota-ledger", body)
	req.RemoteAddr = "10.0.0.1:43210"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := new(admissionv1.AdmissionReview)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), out))
	assert.Equal(t, types.UID("uid-1"), out.Response.UID)
	assert.Equal(t, int32(http.StatusServiceUnavailable), out.Response.Result.Code)
	assert.LessOrEqual(t, body.read, overloadPeekBytes, "the body is not read past the prefix")
}

func TestRequestUID(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		uid        types.UID
	}{
		{name: "the order the API server writes", body: `{"kind":"AdmissionReview","apiVersion":"admission.k8s.io/v1","request":{"uid":"u-1","kind":{"group":""}}}`, uid: "u-1"},
		{name: "nested fields before the uid", body: `{"request":{"kind":{"group":"","uid":"no"},"resource":{"x":[1,{"uid":"no"}]},"uid":"u-2"}}`, uid: "u-2"},
		{name: "a uid outside the request", body: `{"uid":"no","response":{"uid":"no"}}`},
		{name: "a body cut before the uid", body: `{"request":{"kind":{"gro`},
		{name: "not JSON", body: `uid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.uid, requestUID(strings.NewReader(tc.body)))
		})
	}
}

// A decision that misses the deadline is answered busy and records nothing, even when it finishes
// later.
func TestHandlerDeadline(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5"))
	h := newHandler(s, serving)
	s.mu.Lock()
	released := make(chan struct{})
	go func() {
		time.Sleep(3 * handlerDeadline)
		s.mu.Unlock()
		close(released)
	}()
	_, response := call(t, h, reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "5")))
	require.NotNil(t, response)
	assert.False(t, response.Allowed)
	assert.Contains(t, response.Result.Message, "busy")
	<-released
	assert.Eventually(t, func() bool { return s.PendingCount() == 0 }, time.Second, 5*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	assert.Zero(t, s.PendingCount(), "the late decision recorded nothing")
}

func TestHandlerRefusesABadBody(t *testing.T) {
	h := newHandler(NewState(), serving)
	rec, _ := call(t, h, []byte("{"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// A write that names a version the informer has not shown yet waits for it instead of failing on the
// lag of the informer: the quota plugin writes the quota of a namespace one write after the other.
func TestDecideWithRoomWaitsForTheInformer(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "2"), rl("pods", "2"))
	go func() {
		time.Sleep(30 * time.Millisecond)
		s.SetQuota(rq("a", 11, rl("pods", "3")))
	}()
	in := write("a", 11, rl("pods", "3"), rl("pods", "4"))
	h := newHandler(s, serving)
	d := decideWithRoom(context.Background(), s, "a", serving, nil, nil, func() Decision { return h.decide(in) })
	assert.True(t, d.Allowed, d.Message)
}

func TestWaitsForState(t *testing.T) {
	assert.True(t, waitsForState(Decision{Result: ResultLagging, Retry: true}))
	assert.False(t, waitsForState(Decision{Result: ResultInconsistent, Retry: true}), "a uid that does not match: anyone may send one")
	assert.False(t, waitsForState(Decision{Result: ResultInconsistent}), "a usage that does not match")
	assert.False(t, waitsForState(Decision{Result: ResultExceeded}))
}

// canaryBody is a canary write of the status of the canary quota in the namespace, with the nonce.
func canaryBody(t *testing.T, namespace, nonce string) []byte {
	t.Helper()
	dryRun := true
	object, err := json.Marshal(&corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: CanaryName, Namespace: namespace,
		Annotations: map[string]string{CanaryNonceAnnotation: nonce}}})
	require.NoError(t, err)
	body, err := json.Marshal(admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{
		UID: "uid-c", Name: CanaryName, Namespace: namespace, DryRun: &dryRun,
		UserInfo: authenticationv1.UserInfo{Username: ControllerUser}, Object: runtime.RawExtension{Raw: object}, OldObject: runtime.RawExtension{Raw: object},
	}})
	require.NoError(t, err)
	return body
}

func TestHandlerRecordsTheCanary(t *testing.T) {
	h := newHandler(NewState(), serving)
	h.Nonces = &Nonces{}
	_, response := call(t, h, canaryBody(t, Namespace, "n1"))
	require.NotNil(t, response)
	assert.True(t, response.Allowed)
	assert.True(t, h.Nonces.Seen("n1"))

	_, _ = call(t, h, canaryBody(t, "team", "n2"))
	assert.False(t, h.Nonces.Seen("n2"), "a canary quota of another namespace is no canary")

	// a suspension does not stop the API servers from calling the webhook
	h.Suspended = func() bool { return true }
	_, response = call(t, h, canaryBody(t, Namespace, "n3"))
	require.NotNil(t, response)
	assert.True(t, response.Allowed)
	assert.True(t, h.Nonces.Seen("n3"))
}

// Made-up canary requests do not grow the nonces past their bound.
func TestNoncesAreBounded(t *testing.T) {
	n := &Nonces{}
	now := time.Now()
	for i := range 10 * maxNonces {
		n.Record(fmt.Sprintf("forged-%d", i), now.Add(time.Duration(i)*time.Microsecond))
	}
	assert.Len(t, n.seen, maxNonces)
	assert.True(t, n.Seen(fmt.Sprintf("forged-%d", 10*maxNonces-1)), "the newest stays")

	later := now.Add(2 * nonceTTL)
	n.Record("real", later)
	assert.True(t, n.Seen("real"))
	assert.Len(t, n.seen, 1, "the old ones go once the bound is reached")
}

func TestHandlerSuspended(t *testing.T) {
	s := state(rl("pods", "10"), rl("pods", "4"), rl("pods", "5"))
	h := newHandler(s, func() (string, string) { return ResultNotLeader, "test" })
	h.Suspended = func() bool { return true }
	_, response := call(t, h, reviewBody(t, "a", 10, rl("pods", "4"), rl("pods", "50")))
	require.NotNil(t, response)
	assert.True(t, response.Allowed, "a suspended ledger lets every write through")
	assert.Zero(t, s.PendingCount())
}

// The writes of one project that wait for its approvals to be confirmed take the places of their API
// server that may wait: a write of another project from that API server is still answered, not refused
// with an overload, and the calls held, the waiting ones included, stay within perSourceLimit.
func TestWaitingWritesLeaveTheirSource(t *testing.T) {
	s := state(rl("pods", "100"), rl("pods", "1"), rl("pods", "1"))
	s.SetProjectQuota(pq("q", rl("pods", "100")))
	s.SetNamespace(ns("c", "q"))
	s.SetQuota(rq("c", 10, rl("pods", "1")))
	require.True(t, s.Decide(write("a", 10, rl("pods", "1"), rl("pods", "11")), time.Now(), always).Allowed, "the room of p is taken")
	h := newHandler(s, serving)

	var wg sync.WaitGroup
	for range perSourceWaitLimit {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = call(t, h, reviewBody(t, "b", 20, rl("pods", "1"), rl("pods", "2")))
		}()
	}
	require.Eventually(t, func() bool {
		return testutil.ToFloat64(h.Metrics.Inflight.WithLabelValues("10.0.0.1")) == 0 && s.waitingCount() == perSourceWaitLimit
	}, time.Second, 5*time.Millisecond, "the writes of p wait without their places in progress")

	started := time.Now()
	_, response := call(t, h, reviewBody(t, "b", 20, rl("pods", "1"), rl("pods", "2")))
	require.NotNil(t, response)
	assert.False(t, response.Allowed)
	assert.Equal(t, int32(http.StatusServiceUnavailable), response.Result.Code, "a busy answer, which its controller retries")
	assert.Less(t, time.Since(started), inflightWait, "a third write of p does not wait: its source has perSourceWaitLimit waiting")

	_, response = call(t, h, reviewBody(t, "c", 10, rl("pods", "1"), rl("pods", "2")))
	require.NotNil(t, response)
	assert.True(t, response.Allowed, "a write of another project is answered, not refused with an overload")

	assert.True(t, h.enter("10.0.0.1"))
	assert.False(t, h.enter("10.0.0.1"), "the waiting writes count in perSourceLimit")
	h.leave("10.0.0.1")
	wg.Wait()
}

// A namespace the informer has not shown in a project waits on its own: made-up namespaces do not take
// the places of a new namespace of the cluster.
func TestUnknownNamespacesWaitApart(t *testing.T) {
	s := NewState()
	for range maxWaitingPerProject {
		_, ok := s.startWaiting("made-up")
		require.True(t, ok)
	}
	_, ok := s.startWaiting("made-up")
	assert.False(t, ok)
	_, ok = s.startWaiting("new")
	assert.True(t, ok)
}

// waitingCount is the number of writes that wait for room, over all projects.
func (s *State) waitingCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	total := 0
	for _, n := range s.waiting {
		total += n
	}
	return total
}
