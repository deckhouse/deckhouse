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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
	"fencing-controller/internal/domain/fsm"
	"fencing-controller/internal/usecase/profile"
)

const nodeName = "worker-3"

// observedAt is the moment every reconcile below runs at, and the timings are
// those of the medium profile: status timestamps are stored with second
// precision, so the ages below stay whole seconds.
var (
	observedAt = time.Date(2026, time.June, 2, 15, 0, 30, 0, time.UTC)

	medium = fsm.Params{FallbackTTL: 4 * time.Second, EvacuationDelay: 6 * time.Second}
)

func TestReconcileAdvancesThePhase(t *testing.T) {
	for name, tc := range map[string]struct {
		phase            v1alpha1.FencingFailedNodeStatePhase
		failedAgo        time.Duration
		heartbeatAgo     time.Duration
		wantPhase        v1alpha1.FencingFailedNodeStatePhase
		wantRequeueAfter time.Duration
	}{
		"failure inside the delay is suspected": {
			failedAgo:        2 * time.Second,
			wantPhase:        v1alpha1.PhaseSuspected,
			wantRequeueAfter: 4 * time.Second,
		},
		"failure past the delay is ready to evict": {
			failedAgo: 20 * time.Second,
			wantPhase: v1alpha1.PhaseReadyToEvict,
		},
		"a fresh heartbeat holds the eviction back": {
			failedAgo:        20 * time.Second,
			heartbeatAgo:     time.Second,
			wantPhase:        v1alpha1.PhaseFallbackAlive,
			wantRequeueAfter: 3 * time.Second,
		},
		"a stale heartbeat releases it": {
			phase:        v1alpha1.PhaseFallbackAlive,
			failedAgo:    20 * time.Second,
			heartbeatAgo: 20 * time.Second,
			wantPhase:    v1alpha1.PhaseReadyToEvict,
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = tc.phase
			incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-tc.failedAgo))

			if tc.heartbeatAgo > 0 {
				at := metav1.NewTime(observedAt.Add(-tc.heartbeatAgo))
				incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{
					Active:                   true,
					LastHeartbeatAt:          &at,
					HeartbeatIntervalSeconds: 1,
				}
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
			assertCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)
		})
	}
}

// TestReconcileRestoresTheMachineFromThePhase covers a controller that restarted
// mid incident: the phase it finds is where the machine continues from.
func TestReconcileRestoresTheMachineFromThePhase(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseReadyToEvict
	incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-20 * time.Second))

	h := newHarness(t, incident)

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// The eviction path is not implemented yet, so nothing moves the incident on.
	if got := h.get().Status.Phase; got != v1alpha1.PhaseReadyToEvict {
		t.Errorf("phase is %q, want the observed %q", got, v1alpha1.PhaseReadyToEvict)
	}
}

func TestReconcileRejectsAPhaseItCannotRestore(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = "Draining"

	if _, err := newHarness(t, incident).reconcile(); err == nil {
		t.Error("reconcile of an unknown phase succeeded, want an error")
	}
}

// TestReconcileLeavesTheHealthyPhaseUnwritten covers the object the agent created
// before it wrote any evidence: Healthy is not a phase a live object can carry.
func TestReconcileLeavesTheHealthyPhaseUnwritten(t *testing.T) {
	incident := failedState()
	incident.Status = v1alpha1.FencingFailedNodeStateStatus{}

	h := newHarness(t, incident)

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := h.get().Status.Phase; got != "" {
		t.Errorf("phase is %q, want it left unwritten", got)
	}
}

// TestReconcileReportsAnUnusableProfile is the degraded configuration case: the
// incident keeps the phase it had and never reaches the eviction path.
func TestReconcileReportsAnUnusableProfile(t *testing.T) {
	missing := fmt.Errorf("%w: fencingslaprofile %q does not exist", profile.ErrConfiguration, "critical")

	for name, phase := range map[string]v1alpha1.FencingFailedNodeStatePhase{
		"first seen": "",
		"suspected":  v1alpha1.PhaseSuspected,
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			// Old enough that resolved timings would send it to ReadyToEvict.
			incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-20 * time.Second))

			h := newHarness(t, incident)
			h.profiles.err = missing

			if _, err := h.reconcile(); !errors.Is(err, profile.ErrConfiguration) {
				t.Fatalf("reconcile returned %v, want the configuration error so the incident is requeued", err)
			}

			got := h.get()

			if got.Status.Phase != phase {
				t.Errorf("phase moved from %q to %q while the profile was unusable", phase, got.Status.Phase)
			}

			assertCondition(t, got, metav1.ConditionTrue, common.ReasonProfileUnavailable)
			assertAgentSectionsUntouched(t, incident, got)
		})
	}
}

// TestReconcileClearsTheConditionOnceTheProfileIsBack checks the blocker does not
// outlive its cause.
func TestReconcileClearsTheConditionOnceTheProfileIsBack(t *testing.T) {
	incident := failedState()
	incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-2 * time.Second))

	h := newHarness(t, incident)
	h.profiles.err = fmt.Errorf("%w: fencingslaprofile %q does not exist", profile.ErrConfiguration, "critical")

	if _, err := h.reconcile(); err == nil {
		t.Fatal("reconcile succeeded while the profile was missing")
	}

	assertCondition(t, h.get(), metav1.ConditionTrue, common.ReasonProfileUnavailable)

	h.profiles.err = nil

	if _, err := h.reconcile(); err != nil {
		t.Fatalf("reconcile after the profile came back: %v", err)
	}

	got := h.get()

	assertCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)

	if got.Status.Phase != v1alpha1.PhaseSuspected {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseSuspected)
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
}

// TestReconcileWritesNothingTwice keeps a settled incident from generating an
// endless stream of identical status patches.
func TestReconcileWritesNothingTwice(t *testing.T) {
	incident := failedState()
	incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-2 * time.Second))

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

func TestReconcileTreatsMissingObjectAsHealthy(t *testing.T) {
	h := newHarness(t)

	res, err := h.reconcile()
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for a missing object", res)
	}

	// The incident is over, so its resolved timings must not be kept forever.
	if len(h.profiles.forgotten) != 1 || h.profiles.forgotten[0] != nodeName {
		t.Errorf("forgot %v, want the timings of %q to be dropped", h.profiles.forgotten, nodeName)
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

	if _, err := New(c, &stubProfiles{}).Reconcile(t.Context(), request(nodeName)); !errors.Is(err, apiDown) {
		t.Fatalf("reconcile returned %v, want the API error so controller-runtime retries with backoff", err)
	}
}

func TestObservedFieldsAreBalancedPairs(t *testing.T) {
	deleting := failedState()
	deleting.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{Active: true}
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
		"set":        {in: &detectedAt, want: "2026-06-02T15:00:01Z"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := formatTime(tc.in); got != tc.want {
				t.Errorf("formatTime = %q, want %q", got, tc.want)
			}
		})
	}
}

var (
	detectedAt  = metav1.Date(2026, time.June, 2, 15, 0, 1, 0, time.UTC)
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
			NodeGroup:  "worker",
			ProfileRef: v1alpha1.ProfileRef{Name: v1alpha1.ProfileCritical},
		},
		Status: v1alpha1.FencingFailedNodeStateStatus{
			Failed: &v1alpha1.FencingFailedNodeStateFailed{
				DetectedAt: detectedAt,
				DetectedBy: "worker-1",
				Reason:     v1alpha1.FailedReasonMemberlistDead,
				AliveCount: 3,
				QuorumSize: 3,
			},
		},
	}
}

// harness wires the reconciler to an API that rejects every write except the
// status patch the controller owns, and counts those patches.
type harness struct {
	t             *testing.T
	client        client.Client
	reconciler    *Reconciler
	profiles      *stubProfiles
	statusPatches int
}

func newHarness(t *testing.T, objects ...client.Object) *harness {
	t.Helper()

	h := &harness{t: t, profiles: &stubProfiles{params: medium}}

	reject := func(verb string) error {
		t.Errorf("reconcile issued %s, the controller owns the status subresource only", verb)

		return nil
	}

	h.client = fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
				return reject("create")
			},
			Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
				return reject("update")
			},
			Patch: func(context.Context, client.WithWatch, client.Object, client.Patch, ...client.PatchOption) error {
				return reject("patch")
			},
			Delete: func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error {
				return reject("delete")
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
				h.statusPatches++

				return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
			},
		}).
		Build()

	h.reconciler = New(h.client, h.profiles)
	h.reconciler.now = func() time.Time { return observedAt }

	return h
}

func (h *harness) reconcile() (ctrl.Result, error) {
	h.t.Helper()

	return h.reconciler.Reconcile(h.t.Context(), request(nodeName))
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
	params    fsm.Params
	err       error
	forgotten []string
}

func (s *stubProfiles) Resolve(context.Context, *v1alpha1.FencingFailedNodeState) (fsm.Params, error) {
	if s.err != nil {
		return fsm.Params{}, s.err
	}

	return s.params, nil
}

func (s *stubProfiles) Forget(node string) {
	s.forgotten = append(s.forgotten, node)
}

func newScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
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

	if !equality.Semantic.DeepEqual(before.Status.Failed, after.Status.Failed) {
		t.Errorf("failed section changed:\nbefore %+v\nafter  %+v", before.Status.Failed, after.Status.Failed)
	}

	if !equality.Semantic.DeepEqual(before.Status.Fallback, after.Status.Fallback) {
		t.Errorf("fallback section changed:\nbefore %+v\nafter  %+v", before.Status.Fallback, after.Status.Fallback)
	}
}

func assertCondition(
	t *testing.T,
	incident *v1alpha1.FencingFailedNodeState,
	want metav1.ConditionStatus,
	wantReason string,
) {
	t.Helper()

	got := meta.FindStatusCondition(incident.Status.Conditions, common.ConditionTypeConfigurationError)
	if got == nil {
		t.Fatalf("condition %s is not set", common.ConditionTypeConfigurationError)
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

func request(name string) ctrl.Request {
	return ctrl.Request{NamespacedName: types.NamespacedName{Name: name}}
}
