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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/adapters/fencingstate"
	"fencing-controller/internal/adapters/node"
	"fencing-controller/internal/common"
	"fencing-controller/internal/usecase/noderef"
	"fencing-controller/internal/usecase/profile"
)

// The tests below replace the stubs of the reconciler with the collaborators the
// binary is wired with, so the seams between them are covered too: the stubs
// answer whatever a test asks them to and would report the same for any incident
// they are handed, which proves nothing about the object the answer came from.

const nodeUID = types.UID("1a2b3c4d-1111-2222-3333-444455556666")

// TestReconcileActsOnTheProfileTheIncidentNames drives the real resolver over
// real profile objects. The two incidents differ in spec.profileRef.name only,
// so the requeue each of them arms is the proof that the name was read and the
// profile behind it resolved: a resolver that ignored the object would arm the
// same timer for both.
func TestReconcileActsOnTheProfileTheIncidentNames(t *testing.T) {
	for name, tc := range map[string]struct {
		profile v1alpha1.ProfileName
		// want is the rest of evacuation.delay of that profile, counted from a
		// failure detected one second before the incident was observed.
		want time.Duration
	}{
		"critical evacuates in 1.2s": {profile: v1alpha1.ProfileCritical, want: 200 * time.Millisecond},
		"slow evacuates in 45s":      {profile: v1alpha1.ProfileSlow, want: 44 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Spec.ProfileRef.Name = tc.profile
			incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-time.Second))

			res, got := reconcileWithRealCollaborators(t, incident, liveNode())

			if res.RequeueAfter != tc.want {
				t.Errorf("requeued after %s, want %s: the rest of the delay of profile %q",
					res.RequeueAfter, tc.want, tc.profile)
			}

			if got.Status.Phase != v1alpha1.PhaseSuspected {
				t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseSuspected)
			}

			assertProfileCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)
		})
	}
}

// TestReconcileRefusesTheIncidentOfARecreatedNode drives the real validator over
// a real Node object: the incident was created for a Node that no longer exists,
// and the Node that carries its name now is a different one.
func TestReconcileRefusesTheIncidentOfARecreatedNode(t *testing.T) {
	recreated := liveNode()
	recreated.UID = types.UID("99998888-7777-6666-5555-444433332222")

	incident := failedState()
	incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-20 * time.Second))

	_, got := reconcileWithRealCollaborators(t, incident, recreated)

	if got.Status.Phase != v1alpha1.PhaseError {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseError)
	}

	assertCondition(t, got, common.ConditionTypeInvalidNodeReference, metav1.ConditionTrue, common.ReasonUIDMismatch)
}

// TestReconcileRefusesAnIncidentWithoutItsNode covers the terminal path of the
// ADR over the real validator: the Node the incident names is not in the cluster.
func TestReconcileRefusesAnIncidentWithoutItsNode(t *testing.T) {
	incident := failedState()
	incident.Status.Failed.DetectedAt = metav1.NewTime(observedAt.Add(-20 * time.Second))

	res, got := reconcileWithRealCollaborators(t, incident)

	if res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for a node that is gone, want no timer", res)
	}

	if got.Status.Phase != v1alpha1.PhaseError {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseError)
	}

	assertCondition(t, got, common.ConditionTypeInvalidNodeReference, metav1.ConditionTrue, common.ReasonNodeNotFound)
}

// reconcileWithRealCollaborators reconciles the incident once against an API that
// holds the objects given plus every built-in profile, and returns the result and
// the object as it was left.
func reconcileWithRealCollaborators(
	t *testing.T,
	incident *v1alpha1.FencingFailedNodeState,
	objects ...client.Object,
) (ctrl.Result, *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	configurationErrorGauge.Reset()

	c := fake.NewClientBuilder().
		WithScheme(newScheme(t)).
		WithObjects(append(objects, incident)...).
		WithObjects(builtinProfiles()...).
		WithStatusSubresource(&v1alpha1.FencingFailedNodeState{}).
		Build()

	reconciler := New(
		c,
		noderef.NewValidator(node.NewReader(c)),
		profile.NewResolver(fencingstate.NewProfiles(c)),
		record.NewFakeRecorder(eventBuffer),
	)
	reconciler.now = func() time.Time { return observedAt }

	res, err := reconciler.Reconcile(t.Context(), request(incident.Name))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var got v1alpha1.FencingFailedNodeState
	if err := c.Get(t.Context(), types.NamespacedName{Name: incident.Name}, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}

	return res, &got
}

// liveNode is the Node the object of failedState is owned by.
func liveNode() *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: nodeUID}}
}

// builtinProfiles are the four profiles the module ships, with the two timings
// the controller acts on. The values come from the ADR; that the templates carry
// the same ones is checked where the profiles are read.
func builtinProfiles() []client.Object {
	timings := map[v1alpha1.ProfileName][2]time.Duration{
		v1alpha1.ProfileCritical: {time.Second, 1200 * time.Millisecond},
		v1alpha1.ProfileMedium:   {4 * time.Second, 6 * time.Second},
		v1alpha1.ProfileModerate: {10 * time.Second, 20 * time.Second},
		v1alpha1.ProfileSlow:     {20 * time.Second, 45 * time.Second},
	}

	profiles := make([]client.Object, 0, len(timings))

	for name, params := range timings {
		profiles = append(profiles, &v1alpha1.FencingSLAProfile{
			ObjectMeta: metav1.ObjectMeta{Name: name.ObjectName()},
			Spec: v1alpha1.FencingSLAProfileSpec{
				Fallback:   v1alpha1.FencingSLAProfileFallback{TTL: metav1.Duration{Duration: params[0]}},
				Evacuation: v1alpha1.FencingSLAProfileEvacuation{Delay: metav1.Duration{Duration: params[1]}},
			},
		})
	}

	return profiles
}
