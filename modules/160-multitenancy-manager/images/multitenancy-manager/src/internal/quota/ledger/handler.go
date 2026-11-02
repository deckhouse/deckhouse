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
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"controller/internal/quota/evaluator"
)

const (
	// handlerDeadline bounds the work of a call: the quota plugin of an API server has five workers
	// for every create, and a call that a worker waits on must not hold it.
	handlerDeadline = 25 * time.Millisecond
	// inflightWait bounds how long a write waits for the unconfirmed approvals of its project to be
	// confirmed before it is answered busy; with the decisions it fits well within the webhook timeout.
	inflightWait = 250 * time.Millisecond
	// retryWindow is how long a call waits for the informer or for confirmations before it gives a
	// temporary refusal.
	retryWindow = 10 * time.Millisecond
	// perSourceLimit is the most calls of one API server held at a time, the ones that wait for room
	// included, so that at least two of its workers stay free of the webhook.
	perSourceLimit = 3
	// perSourceWaitLimit is the most of them that wait for room, so that a place is always left for the
	// writes of other projects. A further write of one API server that would wait, the third waiting
	// write of the same project among them, is answered 503 (too_many_waiting_source) at once, and its
	// controller retries it.
	perSourceWaitLimit = 2
	// maxReviewBytes bounds the body of a call.
	maxReviewBytes = 3 << 20
	// overloadPeekBytes is how much of a refused body is read for the uid of the request.
	overloadPeekBytes = 4 << 10
	// maxVersionLead is how far above the newest version the informers showed the version of the old
	// object of a write may be: far more versions than a cluster writes while an informer lags.
	maxVersionLead = 1 << 31
)

// GateFunc tells whether the ledger answers now: an empty result when it does, otherwise the result
// and the cause of the refusal.
type GateFunc func() (result, cause string)

// Handler serves the webhook on the status writes of d8-project-quota.
type Handler struct {
	State   *State
	Gate    GateFunc
	Metrics *Metrics
	// Nonces records the canary writes the webhook sees. Optional.
	Nonces *Nonces
	// Suspended reports the quota-ledger-suspend ConfigMap: while it is there, every write is let
	// through without being counted. Optional.
	Suspended func() bool
	// Now is time.Now unless a test sets it.
	Now func() time.Time

	mu       sync.Mutex
	inflight map[string]int
	// waits are the calls of each source that wait for room; they count in perSourceLimit too.
	waits map[string]int
}

// place is what a call of a source holds: its place among the calls in progress, or then among the
// ones that wait. The handler of the call alone uses it.
type place struct {
	h       *Handler
	source  string
	waiting bool
	done    bool
}

// wait moves the call to the ones that wait for room, and reports false when its source has
// perSourceWaitLimit of them already.
func (p *place) wait() bool {
	if p.done || p.waiting || !p.h.toWaiting(p.source) {
		return false
	}
	p.waiting = true
	return true
}

func (p *place) release() {
	if p.done {
		return
	}
	p.done = true
	if p.waiting {
		p.h.stopWait(p.source)
		return
	}
	p.h.leave(p.source)
}

func (h *Handler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	// the place of the source is taken before the body is read, so that the calls of one source, a
	// forged one too, never hold more than perSourceLimit bodies in memory
	source := sourceOf(r.RemoteAddr)
	if !h.enter(source) {
		h.refuseOverload(w, r)
		return
	}
	held := &place{h: h, source: source}
	defer held.release()
	review := new(admissionv1.AdmissionReview)
	body, err := io.ReadAll(io.LimitReader(r.Body, maxReviewBytes))
	if err != nil || json.Unmarshal(body, review) != nil || review.Request == nil {
		h.count(ResultBadRequest)
		http.Error(w, "the body is not an AdmissionReview", http.StatusBadRequest)
		return
	}

	// the canary is answered by the leader alone, also while the ledger is suspended: a suspension does
	// not mean that the API servers stopped calling the webhook
	nonce, isCanary := canaryNonce(review.Request)
	if !isCanary && h.Suspended != nil && h.Suspended() {
		h.respond(w, review, allow(ResultSuspended), false, start)
		return
	}

	if result, cause := h.Gate(); result != "" {
		h.respond(w, review, unavailable(result, cause, false), true, start)
		return
	}

	if isCanary {
		if h.Nonces != nil {
			h.Nonces.Record(nonce, h.now())
		}
		h.respond(w, review, allow(ResultCanary), false, start)
		return
	}

	in, err := inputOf(review.Request)
	if err != nil {
		h.respond(w, review, Decision{Code: http.StatusForbidden, Result: ResultInconsistent, Message: "inconsistent request"}, false, start)
		return
	}
	// the objects of the request are read; a write that waits keeps only the uid of the request
	review.Request.Object.Raw, review.Request.OldObject.Raw = nil, nil
	// a write that waits for room takes one of the places of its source that may wait, so that a few
	// writes of one busy project do not answer the writes of every other project from that API server
	// with an overload
	h.respond(w, review, decideWithRoom(r.Context(), h.State, in.Namespace, h.Gate, h.Metrics, held.wait, func() Decision { return h.decide(in) }), false, start)
}

// decideWithRoom takes a decision and, when the unconfirmed approvals of the project leave no room for
// the write or the informer has not shown what the write replaces yet, waits up to inflightWait for the
// state to change, as when the informer confirms an approval or shows a new version, or a quorum read
// settles one, and decides again. The quota plugin writes the quota of a namespace one write after the
// other, each with the version of the one before, so a create that follows another one in the namespace
// waits for the informer instead of failing on its lag. The share of a limit the unconfirmed approvals may
// take stays as it is: a write gets room only when an approval before it is confirmed. A wait ends early
// when the request ends, and a replica that stopped answering meanwhile, its fence passed, answers so.
// pause, when given, is called before the first wait; when it reports false, the write does not wait.
func decideWithRoom(ctx context.Context, state *State, namespace string, gate GateFunc, metrics *Metrics, pause func() bool,
	decide func() Decision,
) Decision {
	count := func(outcome string) {
		if metrics != nil {
			metrics.InflightWaits.WithLabelValues(outcome).Inc()
		}
	}
	changed := state.Changed()
	d := decide()
	if !waitsForState(d) {
		return d
	}
	project, ok := state.startWaiting(namespace)
	if !ok {
		count("too_many_waiting")
		return d
	}
	defer state.stopWaiting(project)
	if pause != nil && !pause() {
		count("too_many_waiting_source")
		return d
	}
	timer := time.NewTimer(inflightWait)
	defer timer.Stop()
	for {
		select {
		case <-changed:
		case <-ctx.Done():
			count("request_ended")
			return d
		case <-timer.C:
			count("timeout")
			return d
		}
		if result, cause := gate(); result != "" {
			count("not_answering")
			return unavailable(result, cause, false)
		}
		changed = state.Changed()
		if d = decide(); !waitsForState(d) {
			count(d.Result)
			return d
		}
	}
}

// waitsForState reports whether a later state may decide the write otherwise: room for it once the
// approvals before it are confirmed, or the version, the namespace, the project or the quota it names
// once the informers show them.
func waitsForState(d Decision) bool {
	switch d.Result {
	case ResultBusyInflight, ResultLagging, ResultUnknownNamespace, ResultNoProject, ResultNoProjectQuota:
		return true
	}
	// an inconsistent request, a uid that does not match among them, is not waited for: anyone who
	// knows the name of a namespace can send one, and it would take the places of its project that wait
	return false
}

// decide takes the decision within the deadline.
func (h *Handler) decide(in Input) Decision {
	return decideWithin(func(claim func() bool) Decision { return h.State.Decide(in, h.now(), claim) })
}

// decideWithin takes a decision within the deadline. The decision runs aside; whichever of it and the
// deadline comes first answers, and an approval is recorded only when the decision came first, under
// the lock of the ledger: claim says whether it may.
func decideWithin(decide func(claim func() bool) Decision) Decision {
	var answered atomic.Int32 // 0: not yet, 1: by the decision, 2: by the deadline
	claim := func() bool { return answered.CompareAndSwap(0, 1) }

	result := make(chan Decision, 1)
	go func() {
		started := time.Now()
		for {
			d := decide(claim)
			if !d.Retry || time.Since(started) >= retryWindow || answered.Load() != 0 {
				result <- d
				return
			}
			time.Sleep(time.Millisecond)
		}
	}()

	timer := time.NewTimer(handlerDeadline)
	defer timer.Stop()
	select {
	case d := <-result:
		return d
	case <-timer.C:
		if answered.CompareAndSwap(0, 2) {
			return unavailable(ResultBusy, "busy", false)
		}
		return <-result
	}
}

func (h *Handler) respond(w http.ResponseWriter, review *admissionv1.AdmissionReview, d Decision, closeConnection bool, start time.Time) {
	h.respondRaw(w, review, d, closeConnection)
	if h.Metrics != nil {
		h.Metrics.HandlerDuration.Observe(time.Since(start).Seconds())
	}
}

func (h *Handler) respondRaw(w http.ResponseWriter, review *admissionv1.AdmissionReview, d Decision, closeConnection bool) {
	h.count(d.Result)
	if closeConnection {
		// the API server opens a new connection and reaches whichever pod the Service points to now,
		// instead of keeping one to a former leader
		w.Header().Set("Connection", "close")
	}
	response := &admissionv1.AdmissionResponse{Allowed: d.Allowed}
	if review != nil && review.Request != nil {
		response.UID = review.Request.UID
	}
	if !d.Allowed {
		reason := metav1.StatusReasonForbidden
		if d.Code == http.StatusServiceUnavailable {
			reason = metav1.StatusReasonServiceUnavailable
		}
		response.Result = &metav1.Status{Status: metav1.StatusFailure, Code: d.Code, Reason: reason, Message: d.Message}
	}
	out := admissionv1.AdmissionReview{TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"}, Response: response}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		// the connection is gone; the API server times the call out
		return
	}
}

func (h *Handler) count(result string) {
	if h.Metrics != nil {
		h.Metrics.Requests.WithLabelValues(result).Inc()
	}
}

// refuseOverload answers a call of a source that has no place left. The answer carries the uid of the
// request, or the API server takes it for an invalid answer; the uid comes from the first bytes of the
// body, which the API server writes first, so the rest of the body is not read.
func (h *Handler) refuseOverload(w http.ResponseWriter, r *http.Request) {
	review := &admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{UID: requestUID(io.LimitReader(r.Body, overloadPeekBytes))}}
	h.respondRaw(w, review, Decision{Code: http.StatusServiceUnavailable, Result: ResultOverload,
		Message: "project quota ledger unavailable (overload), retry"}, false)
}

// requestUID finds request.uid in the JSON of an AdmissionReview as far as the reader goes, empty when it
// does not get that far.
func requestUID(body io.Reader) types.UID {
	type frame struct {
		object bool
		key    string
		isKey  bool
	}
	decoder := json.NewDecoder(body)
	var path []frame
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		var top *frame
		if len(path) > 0 {
			top = &path[len(path)-1]
		}
		switch t := token.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				path = append(path, frame{object: t == '{', isKey: t == '{'})
			default:
				path = path[:len(path)-1]
				if len(path) > 0 && path[len(path)-1].object {
					path[len(path)-1].isKey = true
				}
			}
			continue
		case string:
			if top != nil && top.object && top.isKey {
				top.key, top.isKey = t, false
				continue
			}
			if len(path) == 2 && path[0].key == "request" && path[1].key == "uid" {
				return types.UID(t)
			}
		}
		if top != nil && top.object {
			top.isKey = true
		}
	}
}

// enter takes a place of the source; it refuses when the source has perSourceLimit calls in progress.
func (h *Handler) enter(source string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.inflight == nil {
		h.inflight = map[string]int{}
	}
	if h.inflight[source]+h.waits[source] >= perSourceLimit {
		return false
	}
	h.inflight[source]++
	if h.Metrics != nil {
		h.Metrics.Inflight.WithLabelValues(source).Set(float64(h.inflight[source]))
	}
	return true
}

func (h *Handler) leave(source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.inflight[source]--
	if h.Metrics != nil {
		h.Metrics.Inflight.WithLabelValues(source).Set(float64(h.inflight[source]))
	}
	if h.inflight[source] == 0 {
		delete(h.inflight, source)
	}
}

// toWaiting moves a call of the source from the calls in progress to the ones that wait.
func (h *Handler) toWaiting(source string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.waits == nil {
		h.waits = map[string]int{}
	}
	if h.waits[source] >= perSourceWaitLimit {
		return false
	}
	h.waits[source]++
	h.inflight[source]--
	if h.Metrics != nil {
		h.Metrics.Inflight.WithLabelValues(source).Set(float64(h.inflight[source]))
	}
	if h.inflight[source] == 0 {
		delete(h.inflight, source)
	}
	return true
}

func (h *Handler) stopWait(source string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.waits[source]--; h.waits[source] <= 0 {
		delete(h.waits, source)
	}
}

func sourceOf(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

// canaryNonce returns the nonce of a canary write: a dry run of the status of the canary quota in the
// namespace of the module by the service account of the module.
func canaryNonce(req *admissionv1.AdmissionRequest) (string, bool) {
	if req.Namespace != Namespace || req.Name != CanaryName || req.UserInfo.Username != ControllerUser || req.DryRun == nil || !*req.DryRun {
		return "", false
	}
	var object struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(req.Object.Raw, &object); err != nil {
		return "", false
	}
	return object.Metadata.Annotations[CanaryNonceAnnotation], true
}

// rqFamily evaluates the quotas the webhook is called for.
var rqFamily evaluator.Evaluator = evaluator.ResourceQuota{}

// inputOf reads the write from the request.
func inputOf(req *admissionv1.AdmissionRequest) (Input, error) {
	in := Input{Namespace: req.Namespace, Name: req.Name, User: req.UserInfo.Username, DryRun: req.DryRun != nil && *req.DryRun}
	if in.DryRun || in.User != APIServerUser {
		return in, nil
	}
	written := new(corev1.ResourceQuota)
	if err := json.Unmarshal(req.Object.Raw, written); err != nil {
		return in, err
	}
	old := new(corev1.ResourceQuota)
	err := json.Unmarshal(req.OldObject.Raw, old)
	if err != nil {
		return in, err
	}
	in.OldUID = old.UID
	in.OldRV = ParseResourceVersion(old.ResourceVersion)
	if in.OldUsed, err = rqFamily.Usage(old); err != nil {
		return in, err
	}
	if in.NewUsed, err = rqFamily.Usage(written); err != nil {
		return in, err
	}
	return in, nil
}
