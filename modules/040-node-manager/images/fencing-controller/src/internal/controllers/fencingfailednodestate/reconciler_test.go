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

package fencingfailednodestate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/domain/fsm"
	"fencing-controller/internal/usecase/profile"
)

const (
	nodeName       = "worker-3"
	agentNamespace = "d8-cloud-instance-manager"
	// eventBuffer holds all events of one test: the fake recorder blocks when
	// the buffer is full.
	eventBuffer = 64
)

// observedAt is the moment of each reconcile below. The heartbeat TTL of the
// timings is 4s and the heartbeat interval is 1s, the same as the critical
// profile.
var (
	observedAt = time.Date(2026, time.June, 2, 15, 0, 30, 0, time.UTC)

	timings = fsm.Params{
		Armed:                  true,
		HeartbeatTTL:           4 * time.Second,
		AttestedNodeTaintAfter: attestedAfter,
		HeartbeatInterval:      time.Second,
		SilentNodeTaintAfter:   silentAfter,
	}
)

// TestReconcileSetsThePhase covers the first reconcile of an incident. The
// times in the object are in the past, but the deadlines start at the
// reconcile.
func TestReconcileSetsThePhase(t *testing.T) {
	for name, tc := range map[string]struct {
		phase            v1alpha1.FencingFailedNodeStatePhase
		heartbeat        bool
		wantPhase        v1alpha1.FencingFailedNodeStatePhase
		wantRequeueAfter time.Duration
	}{
		"a new incident is suspected": {
			wantPhase:        v1alpha1.PhaseSuspected,
			wantRequeueAfter: silentAfter,
		},
		"a heartbeat makes the incident fallback alive": {
			heartbeat:        true,
			wantPhase:        v1alpha1.PhaseFallbackAlive,
			wantRequeueAfter: 4 * time.Second,
		},
		"a suspected incident with a heartbeat is fallback alive": {
			phase:            v1alpha1.PhaseSuspected,
			heartbeat:        true,
			wantPhase:        v1alpha1.PhaseFallbackAlive,
			wantRequeueAfter: 4 * time.Second,
		},
		"a fallback alive incident without a heartbeat is suspected": {
			phase:            v1alpha1.PhaseFallbackAlive,
			wantPhase:        v1alpha1.PhaseSuspected,
			wantRequeueAfter: silentAfter,
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = tc.phase
			incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-time.Hour))

			if tc.heartbeat {
				at := metav1.NewMicroTime(observedAt.Add(-time.Hour))
				incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}
			}

			h := newHarness(t, incident)

			res, err := h.reconcile()
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if res.RequeueAfter != tc.wantRequeueAfter {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, tc.wantRequeueAfter)
			}

			got := h.get()

			if got.Status.Phase != tc.wantPhase {
				t.Errorf("phase is %q, want %q", got.Status.Phase, tc.wantPhase)
			}

			assertAgentSectionsUntouched(t, incident, got)
			assertProfileCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)

			for _, event := range h.events() {
				if !strings.Contains(event, common.ReasonPhaseChanged) {
					t.Errorf("an incident that was never blocked published %q, want the phase events only", event)
				}
			}
		})
	}
}

// TestReconcileCountsTheSilentDelayFromItsOwnObservation covers an incident
// that was detected one hour ago. The silent delay starts when the controller
// sees the object, not at the time in the object.
func TestReconcileCountsTheSilentDelayFromItsOwnObservation(t *testing.T) {
	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-time.Hour))

	h := newHarness(t, incident)

	for _, step := range []struct{ at, want time.Duration }{
		{at: 0, want: silentAfter},
		{at: silentAfter - time.Millisecond, want: time.Millisecond},
	} {
		res := h.reconcileAt(observedAt.Add(step.at))

		if res.RequeueAfter != step.want {
			t.Errorf("requeued after %s at %s, want %s", res.RequeueAfter, step.at, step.want)
		}

		if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
			t.Errorf("phase is %q at %s, want %q", got, step.at, v1alpha1.PhaseSuspected)
		}
	}
}

// TestReconcileReturnsToSuspectedWhenTheHeartbeatStops uses the heartbeat TTL
// of the normal profile: 2 * 2s + 5s = 9s. The controller saw the heartbeat 10s
// ago, so the incident is Suspected again until the silent delay after the
// heartbeat plus heartbeat.interval.
func TestReconcileReturnsToSuspectedWhenTheHeartbeatStops(t *testing.T) {
	incident := failedState()
	at := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	h := newHarness(t, incident)
	h.profiles.params.HeartbeatTTL = 9 * time.Second

	res := h.reconcileAt(observedAt)
	if got := h.get().Status.Phase; got != v1alpha1.PhaseFallbackAlive || res.RequeueAfter != 9*time.Second {
		t.Fatalf("phase is %q with requeue after %s, want %q after 9s", got, res.RequeueAfter, v1alpha1.PhaseFallbackAlive)
	}

	want := time.Second + silentAfter - 10*time.Second

	res = h.reconcileAt(observedAt.Add(10 * time.Second))
	if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected || res.RequeueAfter != want {
		t.Errorf("phase is %q with requeue after %s, want %q after %s", got, res.RequeueAfter, v1alpha1.PhaseSuspected, want)
	}
}

// TestReconcileKeepsFallbackAliveWhileHeartbeatsArrive checks that each new
// heartbeat starts the TTL again from the moment when the controller sees it.
func TestReconcileKeepsFallbackAliveWhileHeartbeatsArrive(t *testing.T) {
	incident := failedState()
	first := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &first}

	h := newHarness(t, incident)
	h.reconcileAt(observedAt)

	// The node writes a new heartbeat. Its time is in the past, but the TTL
	// starts when the controller sees it.
	h.writeHeartbeat(observedAt.Add(-time.Minute))

	res := h.reconcileAt(observedAt.Add(3 * time.Second))
	if res.RequeueAfter != 4*time.Second {
		t.Errorf("requeued after %s, want the full TTL of 4s", res.RequeueAfter)
	}

	res = h.reconcileAt(observedAt.Add(6 * time.Second))
	if got := h.get().Status.Phase; got != v1alpha1.PhaseFallbackAlive || res.RequeueAfter != time.Second {
		t.Errorf("phase is %q with requeue after %s, want %q after 1s", got, res.RequeueAfter, v1alpha1.PhaseFallbackAlive)
	}
}

// TestReconcileOfANewLeaderOnlyDelays covers a change of the leader. The old
// leader saw the heartbeat 3s ago, so 1s of the TTL is left. The new leader
// does not know this. It counts the TTL from its own reconcile, so the incident
// stays FallbackAlive for longer, not for shorter.
func TestReconcileOfANewLeaderOnlyDelays(t *testing.T) {
	incident := failedState()
	at := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	h := newHarness(t, incident)
	h.reconcileAt(observedAt)

	if res := h.reconcileAt(observedAt.Add(3 * time.Second)); res.RequeueAfter != time.Second {
		t.Fatalf("the old leader requeued after %s, want 1s", res.RequeueAfter)
	}

	h.reconciler = New(h.client, h.api, agentNamespace, h.nodes, h.profiles, h.recorder)

	res := h.reconcileAt(observedAt.Add(3 * time.Second))
	if got := h.get().Status.Phase; got != v1alpha1.PhaseFallbackAlive || res.RequeueAfter != 4*time.Second {
		t.Errorf("phase is %q with requeue after %s, want %q after the full TTL of 4s",
			got, res.RequeueAfter, v1alpha1.PhaseFallbackAlive)
	}
}

// TestReconcileForgetsTheObservationsOfARecreatedObject checks that a new
// object with the same name is a new incident.
func TestReconcileForgetsTheObservationsOfARecreatedObject(t *testing.T) {
	incident := failedState()
	at := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	h := newHarness(t, incident)
	h.reconcileAt(observedAt)

	if err := h.api.Delete(h.t.Context(), incident); err != nil {
		t.Fatalf("delete: %v", err)
	}

	recreated := incident.DeepCopy()
	recreated.ResourceVersion = ""
	recreated.UID = types.UID("ccccdddd-1111-2222-3333-444455556666")

	if err := h.api.Create(h.t.Context(), recreated); err != nil {
		t.Fatalf("create: %v", err)
	}

	if res := h.reconcileAt(observedAt.Add(3 * time.Second)); res.RequeueAfter != 4*time.Second {
		t.Errorf("requeued after %s, want the full TTL of 4s for the new object", res.RequeueAfter)
	}
}

// TestReconcileDecidesAgainstOneMoment makes sure that one reconcile reads the
// clock one time. The phase and its deadline must use the same moment.
func TestReconcileDecidesAgainstOneMoment(t *testing.T) {
	incident := failedState()
	at := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	h := newHarness(t, incident)

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if h.clockReads != 1 {
		t.Errorf("reconcile read the clock %d times, want one moment per reconcile", h.clockReads)
	}
}

// TestReconcileKeepsAPhasePastSuspected covers the phases that the controller
// does not change yet.
func TestReconcileKeepsAPhasePastSuspected(t *testing.T) {
	for _, phase := range []v1alpha1.FencingFailedNodeStatePhase{
		v1alpha1.PhaseUnproven,
		v1alpha1.PhaseEvicted,
	} {
		t.Run(string(phase), func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			at := metav1.NewMicroTime(observedAt)
			incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

			h := newHarness(t, incident)

			res, err := h.reconcile()
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if got := h.get().Status.Phase; got != phase {
				t.Errorf("phase is %q, want %q", got, phase)
			}

			if res.RequeueAfter != 0 {
				t.Errorf("requeued after %s, want no timer", res.RequeueAfter)
			}
		})
	}
}

// TestReconcileIgnoresAnObjectOfAnotherNode covers an object that does not
// belong to the Node with its name now: the Node is gone, or a new Node has the
// same name. The controller writes nothing, publishes nothing and sets no
// timer.
func TestReconcileIgnoresAnObjectOfAnotherNode(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseSuspected
	// For the Node of the object, the heartbeat makes the incident
	// FallbackAlive.
	at := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

	h := newHarness(t, incident)
	h.nodes.foreign = true

	res, err := h.reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v, want no timer", res)
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseSuspected)
	}

	if h.statusPatches != 0 {
		t.Errorf("reconcile wrote the status %d times, want none", h.statusPatches)
	}

	if published := h.events(); len(published) != 0 {
		t.Errorf("reconcile published %v, want nothing", published)
	}
}

// TestReconcileChecksTheNodeBeforeTheProfile covers an object of a Node that
// is gone, with a profile that is also not usable. The controller retries a
// profile error without end, so the check of the Node comes first.
func TestReconcileChecksTheNodeBeforeTheProfile(t *testing.T) {
	h := newHarness(t, failedState())
	h.nodes.foreign = true
	h.profiles.err = missingProfile()

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile returned %v, want no retry", err)
	}

	if len(h.profiles.resolved) != 0 {
		t.Errorf("the profile of %v was resolved, want the check of the Node first", h.profiles.resolved)
	}
}

// TestReconcileRetriesWhenTheNodeCannotBeRead covers an API that does not
// answer. The controller retries and writes nothing.
func TestReconcileRetriesWhenTheNodeCannotBeRead(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseSuspected

	h := newHarness(t, incident)
	h.nodeErr = apiDown

	if _, err := h.reconcile(); !errors.Is(err, apiDown) {
		t.Fatalf("reconcile returned %v, want the API error so controller-runtime retries with backoff", err)
	}

	if h.statusPatches != 0 {
		t.Errorf("reconcile wrote the status %d times for a transient failure, want none", h.statusPatches)
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
		t.Errorf("phase moved to %q while the identity of the node was unknown", got)
	}

	if published := h.events(); len(published) != 0 {
		t.Errorf("a transient failure published %v, want nothing blamed on the object", published)
	}
}

// TestReconcileReportsAnUnusableProfile is the degraded configuration case: the
// incident keeps the phase it had and never reaches the eviction path.
func TestReconcileReportsAnUnusableProfile(t *testing.T) {
	for name, phase := range map[string]v1alpha1.FencingFailedNodeStatePhase{
		"first seen": "",
		"suspected":  v1alpha1.PhaseSuspected,
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			// With a usable profile, the heartbeat makes the incident
			// FallbackAlive.
			at := metav1.NewMicroTime(observedAt)
			incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &at}

			h := newHarness(t, incident)
			h.profiles.err = missingProfile()

			if _, err := h.reconcile(); !errors.Is(err, profile.ErrConfiguration) {
				t.Fatalf("reconcile returned %v, want the configuration error so the incident is requeued", err)
			}

			got := h.get()

			if got.Status.Phase != phase {
				t.Errorf("phase moved from %q to %q while the profile was unusable", phase, got.Status.Phase)
			}

			assertProfileCondition(t, got, metav1.ConditionTrue, common.ReasonProfileUnavailable)
			assertAgentSectionsUntouched(t, incident, got)
		})
	}
}

// TestReconcileDoesNotEvictWithoutTheProfile covers a proven incident whose
// profile the controller cannot read. The Node gets no taint and keeps its
// pods.
func TestReconcileDoesNotEvictWithoutTheProfile(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseProven
	attest(incident, observedAt.Add(-time.Hour))

	h := newHarness(t, incident, liveNode(), running(podOn("default", "app")))
	h.profiles.err = missingProfile()

	for _, at := range []time.Time{observedAt, observedAt.Add(time.Hour)} {
		h.reconciler.now = h.clock(at)

		if _, err := h.reconcile(); !errors.Is(err, profile.ErrConfiguration) {
			t.Fatalf("reconcile at %s returned %v, want the configuration error", at, err)
		}
	}

	got := h.get()
	if got.Status.Phase != v1alpha1.PhaseProven {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseProven)
	}

	assertProfileCondition(t, got, metav1.ConditionTrue, common.ReasonProfileUnavailable)
	h.assertNotTainted()
	h.assertPods("default/app")
}

// TestReconcileClearsTheConditionOnceTheProfileIsBack checks the blocker does not
// outlive its cause.
func TestReconcileClearsTheConditionOnceTheProfileIsBack(t *testing.T) {
	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))

	h := newHarness(t, incident)
	h.profiles.err = missingProfile()

	if _, err := h.reconcile(); err == nil {
		t.Fatal("reconcile succeeded while the profile was missing")
	}

	assertProfileCondition(t, h.get(), metav1.ConditionTrue, common.ReasonProfileUnavailable)

	h.profiles.err = nil

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile after the profile came back: %v", err)
	}

	got := h.get()

	assertProfileCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)

	if got.Status.Phase != v1alpha1.PhaseSuspected {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseSuspected)
	}
}

// TestReconcileAnnouncesTheBlockerOnce covers the event published next to
// the condition: it marks the moment fencing became impossible, and the retries
// that follow have nothing to add to it.
func TestReconcileAnnouncesTheBlockerOnce(t *testing.T) {
	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))

	h := newHarness(t, incident)
	h.profiles.err = missingProfile()

	if _, err := h.reconcile(); err == nil {
		t.Fatal("reconcile succeeded while the profile was missing")
	}

	published := h.events()
	if len(published) != 1 {
		t.Fatalf("published %v, want one event naming the blocker", published)
	}

	assertEvent(t, published[0], corev1.EventTypeWarning, common.ReasonProfileUnavailable, "critical")

	if _, err := h.reconcile(); err == nil {
		t.Fatal("second reconcile succeeded while the profile was missing")
	}

	if repeated := h.events(); len(repeated) != 0 {
		t.Errorf("the retry published %v, want the blocker announced once", repeated)
	}
}

// TestReconcileTracksTheConfigurationErrorMetric covers the metric published
// next to the condition, so a blocked node can be alerted on rather than
// found by reading objects.
func TestReconcileTracksTheConfigurationErrorMetric(t *testing.T) {
	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))

	h := newHarness(t, incident)
	h.profiles.err = missingProfile()

	if _, err := h.reconcile(); err == nil {
		t.Fatal("reconcile succeeded while the profile was missing")
	}

	if got := testutil.CollectAndCount(configurationErrorGauge); got != 1 {
		t.Fatalf("the gauge holds %d series, want the blocked node alone", got)
	}

	blocked := configurationErrorGauge.WithLabelValues(nodeName, string(v1alpha1.ProfileCritical))
	if got := testutil.ToFloat64(blocked); got != 1 {
		t.Errorf("the gauge of the blocked node reads %v, want 1", got)
	}

	h.profiles.err = nil

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile after the profile came back: %v", err)
	}

	// The series is dropped rather than zeroed, so a node that is fine again
	// stops being reported at all.
	if got := testutil.CollectAndCount(configurationErrorGauge); got != 0 {
		t.Errorf("the gauge still holds %d series after the profile came back, want none", got)
	}
}

// TestReconcileKeepsTransientProfileErrorsRetryable checks an unavailable API is
// not written to the object as a configuration problem of the operator.
func TestReconcileKeepsTransientProfileErrorsRetryable(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	h := newHarness(t, failedState())
	h.profiles.err = apiDown

	if _, err := h.reconcile(); !errors.Is(err, apiDown) {
		t.Fatalf("reconcile returned %v, want the API error so controller-runtime retries with backoff", err)
	}

	if h.statusPatches != 0 {
		t.Errorf("reconcile wrote the status %d times for a transient failure, want none", h.statusPatches)
	}

	if published := h.events(); len(published) != 0 {
		t.Errorf("a transient failure published %v, want nothing blamed on the configuration", published)
	}

	if got := testutil.CollectAndCount(configurationErrorGauge); got != 0 {
		t.Errorf("a transient failure left %d gauge series, want none", got)
	}
}

// TestReconcileWritesNothingTwice keeps a settled incident from generating an
// endless stream of identical status patches.
func TestReconcileWritesNothingTwice(t *testing.T) {
	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))

	h := newHarness(t, incident)

	for i := range 3 {
		if _, err := h.reconcile(); err != nil {
			t.Fatalf("reconcile %d: %v", i+1, err)
		}
	}

	if h.statusPatches != 1 {
		t.Errorf("three reconciles of one unchanged incident wrote the status %d times, want once", h.statusPatches)
	}
}

func TestReconcileForgetsAMissingObject(t *testing.T) {
	h := newHarness(t)

	reportConfigurationError(nodeName, v1alpha1.ProfileCritical)
	notProgressingGauge.WithLabelValues(nodeName, common.ReasonVolumesAttached).Set(1)

	res, err := h.reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for a missing object", res)
	}

	// The incident is over, so the blocker it was last reported with may not
	// outlive it.
	if got := testutil.CollectAndCount(configurationErrorGauge); got != 0 {
		t.Errorf("the gauge holds %d series for a node with no incident, want none", got)
	}

	assertNotProgressing(t, "")
}

// TestReconcileForgetsADeletedObject covers the object that is on its
// way out but still readable, which is how a deletion looks while the garbage
// collector works through it: the node has no active fencing signal any more, so
// there is nothing left to write to it and nothing to wake up for.
func TestReconcileForgetsADeletedObject(t *testing.T) {
	deletedAt := metav1.NewTime(observedAt.Add(-time.Second))

	incident := failedState()
	incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))
	incident.DeletionTimestamp = &deletedAt
	// The fake client keeps a deleted object around only while something holds
	// it, which is also the only way this state lasts in a real cluster.
	incident.Finalizers = []string{"foregroundDeletion"}

	h := newHarness(t, incident)

	res, err := h.reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for an object that is being deleted", res)
	}

	if h.statusPatches != 0 {
		t.Errorf("reconcile wrote the status of a deleted object %d times, want none", h.statusPatches)
	}

	if len(h.nodes.validated) != 0 {
		t.Errorf("reconcile read the node of a deleted object")
	}
}

// TestReconcileReturnsStatusPatchFailures covers the write of the one part of the
// object the controller owns. A conflict means another writer got there first and
// the decision has to be taken again against the object as it now is. This is
// not an error, so the controller only asks for a reconcile soon. Other
// failures are returned.
func TestReconcileReturnsStatusPatchFailures(t *testing.T) {
	conflict := apierrors.NewConflict(
		schema.GroupResource{Group: v1alpha1.GroupVersion.Group, Resource: "fencingfailednodestates"},
		nodeName,
		errors.New("the object has been modified"),
	)
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	for name, arrange := range map[string]func(h *harness){
		"advancing the phase": func(*harness) {},
		"reporting an unusable profile": func(h *harness) {
			h.profiles.err = missingProfile()
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Spec.DetectedAt = metav1.NewMicroTime(observedAt.Add(-2 * time.Second))

			h := newHarness(t, incident)
			h.statusPatchErr = conflict

			arrange(h)

			res, err := h.reconcile()
			if err != nil || res.RequeueAfter <= 0 || res.RequeueAfter >= recheckAfter {
				t.Fatalf("reconcile returned %+v, %v, want a requeue sooner than %s and no error after the conflict", res, err, recheckAfter)
			}

			h.statusPatchErr = apiDown

			if _, err := h.reconcile(); !errors.Is(err, apiDown) {
				t.Errorf("reconcile returned %v, want the failure of the write", err)
			}

			if h.statusPatches != 2 {
				t.Errorf("2 reconciles attempted the status patch %d times, want one attempt per pass", h.statusPatches)
			}

			// Announcing a blocker that was never recorded would leave an event
			// no condition backs up.
			if published := h.events(); len(published) != 0 {
				t.Errorf("a failed write published %v, want nothing announced", published)
			}
		})
	}
}

func TestReconcileReturnsAPIErrors(t *testing.T) {
	apiDown := apierrors.NewServiceUnavailable("etcd leader changed")

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
				return apiDown
			},
		}).
		Build()

	reconciler := New(c, c, agentNamespace, &stubNodes{}, &stubProfiles{}, record.NewFakeRecorder(eventBuffer))

	if _, err := reconciler.Reconcile(t.Context(), request(nodeName)); !errors.Is(err, apiDown) {
		t.Fatalf("reconcile returned %v, want the API error so controller-runtime retries with backoff", err)
	}
}

func TestObservedFieldsAreBalancedPairs(t *testing.T) {
	deleting := failedState()
	heartbeat := metav1.NewMicroTime(heartbeatAt.Time)
	deleting.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}
	deleting.DeletionTimestamp = &heartbeatAt

	for name, state := range map[string]*v1alpha1.FencingFailedNodeState{
		"every section set": deleting,
		"zero value":        {},
	} {
		t.Run(name, func(t *testing.T) {
			fields := observedFields(state)

			if len(fields)%2 != 0 {
				t.Fatalf("observedFields returned %d values, want key-value pairs", len(fields))
			}

			for i := 0; i < len(fields); i += 2 {
				if _, ok := fields[i].(string); !ok {
					t.Errorf("key at index %d is %T, want string", i, fields[i])
				}
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	for name, tc := range map[string]struct {
		in   *metav1.Time
		want string
	}{
		"unset":      {in: nil, want: ""},
		"zero value": {in: &metav1.Time{}, want: ""},
		"set":        {in: &heartbeatAt, want: "2026-06-02T15:00:02Z"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := formatTime(tc.in); got != tc.want {
				t.Errorf("formatTime = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatMicroTime(t *testing.T) {
	for name, tc := range map[string]struct {
		in   *metav1.MicroTime
		want string
	}{
		"unset":      {in: nil, want: ""},
		"zero value": {in: &metav1.MicroTime{}, want: ""},
		"set":        {in: &detectedAt, want: "2026-06-02T15:00:01.250000Z"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := formatMicroTime(tc.in); got != tc.want {
				t.Errorf("formatMicroTime = %q, want %q", got, tc.want)
			}
		})
	}
}

var (
	detectedAt  = metav1.NewMicroTime(time.Date(2026, time.June, 2, 15, 0, 1, 250000000, time.UTC))
	heartbeatAt = metav1.Date(2026, time.June, 2, 15, 0, 2, 0, time.UTC)
)

func failedState() *v1alpha1.FencingFailedNodeState {
	return &v1alpha1.FencingFailedNodeState{
		ObjectMeta: metav1.ObjectMeta{
			Name:       nodeName,
			Generation: 1,
			UID:        types.UID("aaaabbbb-1111-2222-3333-444455556666"),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Node",
				Name:       nodeName,
				UID:        types.UID("1a2b3c4d-1111-2222-3333-444455556666"),
			}},
		},
		Spec: v1alpha1.FencingFailedNodeStateSpec{
			Source:     v1alpha1.SourceMemberlistDead,
			DetectedBy: "worker-1",
			DetectedAt: detectedAt,
			BootID:     "0f1e2d3c-1111-2222-3333-444455556666",
			Armed:      true,
			ProfileRef: &v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
	}
}

// harness wires the reconciler to an API that rejects every write except the
// status patch the controller owns, and counts those patches. It also lets the
// controller set the status of a pod.
type harness struct {
	t      *testing.T
	client client.Client
	// api writes to the same objects as client, without the checks of the
	// harness. The tests use it to act as the other writers.
	api client.Client
	// reader is the reader of the reconciler. It reads api.
	reader        client.Reader
	reconciler    *Reconciler
	nodes         *stubNodes
	profiles      *stubProfiles
	recorder      *record.FakeRecorder
	statusPatches int
	nodePatches   int
	clockReads    int
	// podDeletions are the options of each deletion of a pod. The controller
	// deletes pods at the same time, so mu guards the list.
	mu           sync.Mutex
	podDeletions []client.DeleteOptions
	// beforePodDelete, when set, runs before the API deletes a pod.
	beforePodDelete func()
	// podDeleteErr, when set, is what the API answers the deletion of a pod
	// with.
	podDeleteErr error
	// replaceErr, when set, is what the API answers the deletion of a Node or
	// a Machine with.
	replaceErr error
	// attachmentsErr, when set, is what the API answers the list of the
	// VolumeAttachments with.
	attachmentsErr error
	// nodeErr, when set, is what the API answers the read of a Node with.
	nodeErr error
	// beforeStatusPatch, when set, runs before the API applies a status
	// patch. The tests use it to act as another writer between the read and
	// the write of the controller.
	beforeStatusPatch func()
	// statusPatchErr, when set, is what the API answers the status patch with.
	statusPatchErr error
}

func newHarness(t *testing.T, objects ...client.Object) *harness {
	t.Helper()

	resetMetrics()

	h := &harness{
		t:        t,
		nodes:    &stubNodes{},
		profiles: &stubProfiles{params: timings},
		recorder: record.NewFakeRecorder(eventBuffer),
	}

	reject := func(verb string) error {
		t.Errorf("reconcile issued %s, the controller owns the status subresource only", verb)

		return nil
	}

	h.api = newFakeAPI(t, objects...)

	h.client = interceptor.NewClient(h.api.(client.WithWatch), interceptor.Funcs{
		Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
			return reject("create")
		},
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
			return reject("update")
		},
		// The controller sets the taint on a Node and deletes the pods of
		// the Node.
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if _, ok := obj.(*corev1.Node); !ok {
				return reject("patch")
			}

			h.nodePatches++

			return c.Patch(ctx, obj, patch, opts...)
		},
		// The controller deletes the pods of the Node, the object of a Node
		// that came back, and the Node or the Machine of a cloud machine.
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			switch obj.(type) {
			case *corev1.Pod:
				h.mu.Lock()
				h.podDeletions = append(h.podDeletions, *(&client.DeleteOptions{}).ApplyOptions(opts))
				h.mu.Unlock()

				if h.beforePodDelete != nil {
					h.beforePodDelete()
				}

				if h.podDeleteErr != nil {
					return h.podDeleteErr
				}
			case *corev1.Node, *unstructured.Unstructured:
				if h.replaceErr != nil {
					return h.replaceErr
				}
			case *v1alpha1.FencingFailedNodeState:
			default:
				return reject("delete")
			}

			return c.Delete(ctx, obj, opts...)
		},
		DeleteAllOf: func(context.Context, client.WithWatch, client.Object, ...client.DeleteAllOfOption) error {
			return reject("deleteAllOf")
		},
		SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
			return reject("status update")
		},
		SubResourcePatch: func(
			ctx context.Context,
			c client.Client,
			subResource string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			// The controller marks a pod of the dead Node Failed.
			if _, ok := obj.(*corev1.Pod); ok {
				return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
			}

			h.statusPatches++

			if h.statusPatchErr != nil {
				return h.statusPatchErr
			}

			if h.beforeStatusPatch != nil {
				h.beforeStatusPatch()
			}

			return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
		},
	})

	h.reader = interceptor.NewClient(h.api.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Node); ok && h.nodeErr != nil {
				return h.nodeErr
			}

			return c.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*storagev1.VolumeAttachmentList); ok && h.attachmentsErr != nil {
				return h.attachmentsErr
			}

			return c.List(ctx, list, opts...)
		},
	})

	h.reconciler = New(h.client, h.reader, agentNamespace, h.nodes, h.profiles, h.recorder)
	h.reconciler.now = h.clock(observedAt)

	return h
}

// resetMetrics clears the gauges. They are package-level collectors, so every
// test starts from clean ones instead of reading what its predecessors left
// behind.
func resetMetrics() {
	configurationErrorGauge.Reset()
	aliveAfterAttestationGauge.Reset()
	phaseGauge.Reset()
	outOfServiceTaintedGauge.Reset()
	notProgressingGauge.Reset()
}

// clock hands out the same moment every time and counts how often it was asked,
// so a test can pin how many moments one reconcile decides against.
func (h *harness) clock(at time.Time) func() time.Time {
	return func() time.Time {
		h.clockReads++

		return at
	}
}

func (h *harness) reconcile() (ctrl.Result, error) {
	h.t.Helper()

	return h.reconciler.Reconcile(h.t.Context(), request(nodeName))
}

// reconcileAt reconciles the incident at the given moment and fails the test
// on an error.
func (h *harness) reconcileAt(at time.Time) ctrl.Result {
	h.t.Helper()

	h.reconciler.now = h.clock(at)

	res, err := h.reconcile()
	if err != nil {
		h.t.Fatalf("reconcile at %s: %v", at, err)
	}

	return res
}

// writeHeartbeat writes a heartbeat to the object, as the failed node does.
func (h *harness) writeHeartbeat(at time.Time) {
	h.t.Helper()

	incident := h.get()
	heartbeat := metav1.NewMicroTime(at)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}

	if err := h.api.Status().Update(h.t.Context(), incident); err != nil {
		h.t.Fatalf("write heartbeat: %v", err)
	}
}

// writeContact writes a contact with the failed node, as a peer does.
func (h *harness) writeContact(at time.Time) {
	h.t.Helper()

	incident := h.get()
	incident.Status.Contact = &v1alpha1.FencingFailedNodeStateContact{At: metav1.NewMicroTime(at), ReportedBy: "worker-2"}

	if err := h.api.Status().Update(h.t.Context(), incident); err != nil {
		h.t.Fatalf("write contact: %v", err)
	}
}

// events drains what the reconciler published, as "Type Reason message" lines.
func (h *harness) events() []string {
	h.t.Helper()

	var published []string

	for {
		select {
		case event := <-h.recorder.Events:
			published = append(published, event)
		default:
			return published
		}
	}
}

func (h *harness) get() *v1alpha1.FencingFailedNodeState {
	h.t.Helper()

	var got v1alpha1.FencingFailedNodeState
	if err := h.client.Get(h.t.Context(), types.NamespacedName{Name: nodeName}, &got); err != nil {
		h.t.Fatalf("get after reconcile: %v", err)
	}

	return &got
}

type stubProfiles struct {
	params   fsm.Params
	err      error
	resolved []string
}

func (s *stubProfiles) Resolve(_ context.Context, incident *v1alpha1.FencingFailedNodeState) (fsm.Params, error) {
	s.resolved = append(s.resolved, incident.Name)

	if s.err != nil {
		return fsm.Params{Profile: profileName(incident)}, s.err
	}

	// The same as the resolver: the timings tell if the node is armed, and
	// the profile of the incident gives them.
	params := s.params
	params.Armed = incident.Spec.Armed
	params.Profile = profileName(incident)

	return params, nil
}

// newFakeAPI is the API server of the tests. The pods have the index of
// spec.nodeName, so the field selector of the controller works.
func newFakeAPI(t *testing.T, objects ...client.Object) client.WithWatch {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}, &corev1.Pod{}).
		WithIndex(&corev1.Pod{}, podNodeNameField, func(obj client.Object) []string {
			return []string{obj.(*corev1.Pod).Spec.NodeName}
		}).
		Build()
}

// stubNodes tells that each incident belongs to its Node, unless a test says
// otherwise.
type stubNodes struct {
	foreign   bool
	validated []string
}

func (s *stubNodes) Identifies(incident *v1alpha1.FencingFailedNodeState, _ *corev1.Node) bool {
	s.validated = append(s.validated, incident.Name)

	return !s.foreign
}

// newScheme registers what the controller reads, which is the fencing API and
// the core objects, exactly as the manager does.
func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("register client-go scheme: %v", err)
	}

	if err := v1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("add fencing API to scheme: %v", err)
	}

	return s
}

// assertAgentSectionsUntouched checks the controller stayed out of the parts of
// the object other writers own: the immutable spec and the evidence sections.
func assertAgentSectionsUntouched(t *testing.T, before, after *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	if !equality.Semantic.DeepEqual(before.Spec, after.Spec) {
		t.Errorf("spec changed:\nbefore %+v\nafter  %+v", before.Spec, after.Spec)
	}

	if !equality.Semantic.DeepEqual(before.Status.Attestation, after.Status.Attestation) {
		t.Errorf("attestation section changed:\nbefore %+v\nafter  %+v", before.Status.Attestation, after.Status.Attestation)
	}

	if !equality.Semantic.DeepEqual(before.Status.Fallback, after.Status.Fallback) {
		t.Errorf("fallback section changed:\nbefore %+v\nafter  %+v", before.Status.Fallback, after.Status.Fallback)
	}
}

// missingProfile is what the resolver reports when the built-in profile of an
// incident is not in the cluster.
func missingProfile() error {
	return fmt.Errorf("%w: fencingslaprofile %q does not exist", profile.ErrConfiguration, "critical")
}

// assertEvent reads one line of the fake recorder, formatted as
// "Type Reason message". The message must start with the prefix [fencing].
func assertEvent(t *testing.T, event, wantType, wantReason string, wantInMessage ...string) {
	t.Helper()

	for _, want := range append([]string{wantType, wantReason + " " + eventPrefix}, wantInMessage...) {
		if !strings.Contains(event, want) {
			t.Errorf("event %q does not mention %q", event, want)
		}
	}
}

func assertCondition(
	t *testing.T,
	incident *v1alpha1.FencingFailedNodeState,
	conditionType string,
	want metav1.ConditionStatus,
	wantReason string,
) {
	t.Helper()

	got := meta.FindStatusCondition(incident.Status.Conditions, conditionType)
	if got == nil {
		t.Fatalf("condition %s is not set", conditionType)
	}

	if got.Status != want {
		t.Errorf("condition %s is %q, want %q", got.Type, got.Status, want)
	}

	if got.Reason != wantReason {
		t.Errorf("condition %s has reason %q, want %q", got.Type, got.Reason, wantReason)
	}

	if got.ObservedGeneration != incident.Generation {
		t.Errorf("condition %s observed generation %d, want %d", got.Type, got.ObservedGeneration, incident.Generation)
	}
}

func assertProfileCondition(
	t *testing.T,
	incident *v1alpha1.FencingFailedNodeState,
	want metav1.ConditionStatus,
	wantReason string,
) {
	t.Helper()

	assertCondition(t, incident, common.ConditionTypeConfigurationError, want, wantReason)
}

func request(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}
