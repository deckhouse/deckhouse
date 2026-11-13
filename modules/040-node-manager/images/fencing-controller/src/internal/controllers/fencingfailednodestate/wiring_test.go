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

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/adapters/fencingstate"
	"fencing-controller/internal/common"
	"fencing-controller/internal/usecase/noderef"
	"fencing-controller/internal/usecase/profile"
)

// The tests below use the real validator, resolver and adapters, the same as
// the binary uses. The stubs of the other tests give the same answer for each
// incident. Thus only these tests show that the answer comes from the object.

const nodeUID = types.UID("1a2b3c4d-1111-2222-3333-444455556666")

// TestReconcileActsOnTheProfileOfTheIncident uses real profile objects. The
// incidents have different profiles. Each requeue is the heartbeat TTL of the
// profile, so it shows which profile the controller read. An unarmed node has
// no spec.profileRef. Its heartbeat TTL is 20s, and the API has no ConfigMap of
// the agents.
func TestReconcileActsOnTheProfileOfTheIncident(t *testing.T) {
	for name, tc := range map[string]struct {
		profile v1alpha1.ProfileName
		// want is the heartbeat TTL of the profile: 2 * heartbeat.interval +
		// heartbeat.apiTimeout.
		want time.Duration
	}{
		"armed critical": {profile: v1alpha1.ProfileCritical, want: 4 * time.Second},
		"armed slow":     {profile: v1alpha1.ProfileSlow, want: 20 * time.Second},
		"unarmed":        {want: 20 * time.Second},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			if tc.profile == "" {
				incident.Spec.Armed = false
				incident.Spec.ProfileRef = nil
			} else {
				incident.Spec.ProfileRef.Name = tc.profile
			}

			heartbeat := metav1.NewMicroTime(observedAt.Add(-time.Hour))
			incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}

			res, got := reconcileWithRealAdapters(t, incident, liveNode())

			if res.RequeueAfter != tc.want {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, tc.want)
			}

			if got.Status.Phase != v1alpha1.PhaseFallbackAlive {
				t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseFallbackAlive)
			}

			assertProfileCondition(t, got, metav1.ConditionFalse, common.ReasonProfileResolved)
		})
	}
}

// TestReconcileTaintsAfterTheWatchdogOfTheProfile uses real profile objects.
// The taint comes watchdog.timeout + watchdog.margin of spec.profileRef after
// the controller sees the attestation. The group of the Node is critical now,
// but the controller uses the profile of the object.
func TestReconcileTaintsAfterTheWatchdogOfTheProfile(t *testing.T) {
	for name, tc := range map[v1alpha1.ProfileName]time.Duration{
		v1alpha1.ProfileCritical: 5300 * time.Millisecond,
		v1alpha1.ProfileNormal:   15750 * time.Millisecond,
		v1alpha1.ProfileSlow:     31500 * time.Millisecond,
	} {
		t.Run(string(name), func(t *testing.T) {
			incident := failedState()
			incident.Spec.ProfileRef.Name = name
			attest(incident, observedAt.Add(-time.Hour))

			node := liveNode()
			node.Labels = map[string]string{"node.deckhouse.io/group": "critical-workers"}

			reconciler, c := newRealReconciler(t, incident, node)

			if res, _ := reconcileOnce(t, reconciler, c, nodeName, observedAt); res.RequeueAfter != tc {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, tc)
			}

			if _, got := reconcileOnce(t, reconciler, c, nodeName, observedAt.Add(tc-time.Millisecond)); got.Status.Phase != v1alpha1.PhaseSuspected {
				t.Errorf("phase is %q before %s, want %q", got.Status.Phase, tc, v1alpha1.PhaseSuspected)
			}

			if _, got := reconcileOnce(t, reconciler, c, nodeName, observedAt.Add(tc)); got.Status.Phase != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q at %s, want %q", got.Status.Phase, tc, v1alpha1.PhaseEvicted)
			}

			var tainted corev1.Node
			if err := c.Get(t.Context(), types.NamespacedName{Name: nodeName}, &tainted); err != nil {
				t.Fatalf("get node: %v", err)
			}

			if len(tainted.Spec.Taints) != 1 || !tainted.Spec.Taints[0].MatchTaint(&outOfServiceTaint) ||
				tainted.Spec.Taints[0].Value != "nodeshutdown" {
				t.Errorf("taints are %+v, want node.kubernetes.io/out-of-service=nodeshutdown:NoExecute", tainted.Spec.Taints)
			}
		})
	}
}

// TestReconcileTaintsASilentNodeAfterTheSilentDelayOfTheProfile uses real
// profile objects. The node has no attestation and no heartbeat. The taint
// comes controlPlaneLostAfter + 200ms + watchdog.timeout + watchdog.margin +
// 900ms after the controller saw the object.
func TestReconcileTaintsASilentNodeAfterTheSilentDelayOfTheProfile(t *testing.T) {
	for name, silent := range map[v1alpha1.ProfileName]time.Duration{
		v1alpha1.ProfileCritical: 10550 * time.Millisecond,
		v1alpha1.ProfileNormal:   25950 * time.Millisecond,
		v1alpha1.ProfileSlow:     52800 * time.Millisecond,
	} {
		t.Run(string(name), func(t *testing.T) {
			incident := failedState()
			incident.Spec.ProfileRef.Name = name

			reconciler, c := newRealReconciler(t, incident, liveNode())

			if res, _ := reconcileOnce(t, reconciler, c, nodeName, observedAt); res.RequeueAfter != silent {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, silent)
			}

			if _, got := reconcileOnce(t, reconciler, c, nodeName, observedAt.Add(silent-time.Millisecond)); got.Status.Phase != v1alpha1.PhaseSuspected {
				t.Errorf("phase is %q before %s, want %q", got.Status.Phase, silent, v1alpha1.PhaseSuspected)
			}

			if _, got := reconcileOnce(t, reconciler, c, nodeName, observedAt.Add(silent)); got.Status.Phase != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q at %s, want %q", got.Status.Phase, silent, v1alpha1.PhaseEvicted)
			}

			var tainted corev1.Node
			if err := c.Get(t.Context(), types.NamespacedName{Name: nodeName}, &tainted); err != nil {
				t.Fatalf("get node: %v", err)
			}

			if len(tainted.Spec.Taints) != 1 || !tainted.Spec.Taints[0].MatchTaint(&outOfServiceTaint) {
				t.Errorf("taints are %+v, want the out-of-service taint", tainted.Spec.Taints)
			}
		})
	}
}

// TestReconcileDoesNotEvictARecreatedNode uses the real validator. The witness
// attested the old Node long ago, so with a valid reference each phase leads to
// the eviction. The Node with this name now is a different Node, so the
// controller does not set the taint and does not delete the pods or the Node.
func TestReconcileDoesNotEvictARecreatedNode(t *testing.T) {
	for _, phase := range []v1alpha1.FencingFailedNodeStatePhase{
		v1alpha1.PhaseSuspected,
		v1alpha1.PhaseProven,
		v1alpha1.PhaseEvicting,
	} {
		t.Run(string(phase), func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			attest(incident, observedAt.Add(-time.Hour))

			recreated := liveNode()
			recreated.UID = types.UID("99998888-7777-6666-5555-444433332222")

			h := newHarness(t, incident, recreated, running(podOn("default", "app")))
			h.reconciler.nodes = noderef.NewValidator()

			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(time.Hour))

			got := h.get()
			if got.Status.Phase != phase {
				t.Errorf("phase is %q, want %q", got.Status.Phase, phase)
			}

			h.assertNotTainted()
			h.assertPods("default/app")

			if len(h.podDeletions) != 0 {
				t.Errorf("deleted pods %d times, want no deletion", len(h.podDeletions))
			}

			if h.node().UID != recreated.UID {
				t.Errorf("node has UID %q, want the recreated node %q", h.node().UID, recreated.UID)
			}
		})
	}
}

// TestReconcileSendsTheReplacementOfAnEvictedNodeAgain uses the real validator.
// For an Evicted incident the controller sends the deletion that gets a new
// machine again. It does not delete a recreated Node with the same name, a
// master or a static CAPI machine. When the MachineHealthChecks do not replace
// the machine now, it examines the Node again after recheckAfter.
func TestReconcileSendsTheReplacementOfAnEvictedNodeAgain(t *testing.T) {
	recreated := func(n *corev1.Node) {
		capiNode("dvp")(n)
		n.UID = types.UID("99998888-7777-6666-5555-444433332222")
	}

	master := func(n *corev1.Node) {
		capiNode("dvp")(n)
		n.Labels = map[string]string{"node-role.kubernetes.io/control-plane": ""}
	}

	for name, tc := range map[string]struct {
		node        func(*corev1.Node)
		healthy     int64
		deleted     bool
		wantRequeue time.Duration
	}{
		"capi, the machinehealthcheck replaces the machine": {node: capiNode("dvp"), healthy: 3, deleted: true},
		"capi, 3 of 4 unhealthy":                            {node: capiNode("dvp"), healthy: 1, wantRequeue: recheckAfter},
		"recreated node with the same name":                 {node: recreated, healthy: 3},
		"master":                                            {node: master, healthy: 3},
		"static capi machine":                               {node: capiNode(staticClusterName), healthy: 3},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = v1alpha1.PhaseEvicted
			attest(incident, observedAt.Add(-time.Hour))

			n := liveNode()
			tc.node(n)

			h := newHarness(t, incident, n, healthCheck("dvp", 4, tc.healthy), healthCheck(staticClusterName, 4, 4))
			h.reconciler.nodes = noderef.NewValidator()

			if res := h.reconcileAt(observedAt); res.RequeueAfter != tc.wantRequeue {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, tc.wantRequeue)
			}

			if got := h.exists(&corev1.Node{}, "", nodeName); got == tc.deleted {
				t.Errorf("node exists: %t, want %t", got, !tc.deleted)
			}

			if got := h.get().Status.Phase; !tc.deleted && got != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
			}
		})
	}
}

// TestReconcileIgnoresAnIncidentWithoutItsNode uses the real validator. The
// controller deleted the Node of an evicted cloud machine, and the garbage
// collector did not delete the object yet. The controller writes nothing and
// publishes nothing.
func TestReconcileIgnoresAnIncidentWithoutItsNode(t *testing.T) {
	incident := failedState()
	incident.Status.Phase = v1alpha1.PhaseEvicted
	attest(incident, observedAt.Add(-time.Hour))

	h := newHarness(t, incident)
	h.reconciler.nodes = noderef.NewValidator()

	if res := h.reconcileAt(observedAt); res != (ctrl.Result{}) {
		t.Errorf("reconcile requeued %+v for a node that is gone, want no timer", res)
	}

	if h.statusPatches != 0 {
		t.Errorf("reconcile wrote the status %d times, want none", h.statusPatches)
	}

	if published := h.events(); len(published) != 0 {
		t.Errorf("reconcile published %v, want nothing", published)
	}
}

// reconcileWithRealAdapters reconciles the incident one time. The API has
// the given objects and the built-in profiles. The function returns the result
// and the object after the reconcile.
func reconcileWithRealAdapters(
	t *testing.T,
	incident *v1alpha1.FencingFailedNodeState,
	objects ...client.Object,
) (ctrl.Result, *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	reconciler, c := newRealReconciler(t, append(objects, incident)...)

	return reconcileOnce(t, reconciler, c, incident.Name, observedAt)
}

// newRealReconciler returns the reconciler with the real adapters and its API.
// The API has the given objects and the built-in profiles.
func newRealReconciler(t *testing.T, objects ...client.Object) (*Reconciler, client.Client) {
	t.Helper()

	configurationErrorGauge.Reset()
	aliveAfterAttestationGauge.Reset()

	c := newFakeAPI(t, append(objects, builtinProfiles()...)...)

	reconciler := New(
		c,
		c,
		agentNamespace,
		noderef.NewValidator(),
		profile.NewResolver(fencingstate.NewProfiles(c)),
		record.NewFakeRecorder(eventBuffer),
	)

	return reconciler, c
}

// reconcileOnce reconciles the incident at the given moment. It returns the
// result and the object after the reconcile.
func reconcileOnce(
	t *testing.T,
	reconciler *Reconciler,
	c client.Client,
	name string,
	at time.Time,
) (ctrl.Result, *v1alpha1.FencingFailedNodeState) {
	t.Helper()

	reconciler.now = func() time.Time { return at }

	res, err := reconciler.Reconcile(t.Context(), request(name))
	if err != nil {
		t.Fatalf("reconcile at %s: %v", at, err)
	}

	var got v1alpha1.FencingFailedNodeState
	if err := c.Get(t.Context(), types.NamespacedName{Name: name}, &got); err != nil {
		t.Fatalf("get after reconcile: %v", err)
	}

	return res, &got
}

// liveNode is the Node the object of failedState is owned by. Its agent wrote
// that the watchdog is armed.
func liveNode() *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, UID: nodeUID},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{armedCondition()}},
	}
}

// armedCondition is the condition Fencing that the agent of an armed node
// writes.
func armedCondition() corev1.NodeCondition {
	return corev1.NodeCondition{Type: fencingConditionType, Status: corev1.ConditionTrue, Reason: "Armed"}
}

// builtinProfiles are the three built-in profiles with the timings that the
// controller reads. Each profile has its own heartbeat and watchdog timings, so
// a test can see which profile the controller read.
func builtinProfiles() []client.Object {
	// heartbeat.interval, heartbeat.apiTimeout, watchdog.timeout,
	// watchdog.margin and controlPlaneLostAfter.
	timings := map[v1alpha1.ProfileName][5]time.Duration{
		v1alpha1.ProfileCritical: {time.Second, 2 * time.Second, 5 * time.Second, 300 * time.Millisecond, 4150 * time.Millisecond},
		v1alpha1.ProfileNormal:   {2 * time.Second, 5 * time.Second, 15 * time.Second, 750 * time.Millisecond, 9100 * time.Millisecond},
		v1alpha1.ProfileSlow:     {5 * time.Second, 10 * time.Second, 30 * time.Second, 1500 * time.Millisecond, 20200 * time.Millisecond},
	}

	profiles := make([]client.Object, 0, len(timings))

	for name, timing := range timings {
		profiles = append(profiles, &v1alpha1.FencingSLAProfile{
			ObjectMeta: metav1.ObjectMeta{Name: name.ObjectName()},
			Spec: v1alpha1.FencingSLAProfileSpec{
				ControlPlaneLostAfter: metav1.Duration{Duration: timing[4]},
				Watchdog: v1alpha1.FencingSLAProfileWatchdog{
					Timeout: metav1.Duration{Duration: timing[2]},
					Margin:  metav1.Duration{Duration: timing[3]},
				},
				Heartbeat: v1alpha1.FencingSLAProfileHeartbeat{
					Interval:   metav1.Duration{Duration: timing[0]},
					APITimeout: metav1.Duration{Duration: timing[1]},
				},
			},
		})
	}

	return profiles
}
