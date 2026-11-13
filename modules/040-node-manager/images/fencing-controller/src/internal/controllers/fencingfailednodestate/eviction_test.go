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
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// The timings of the harness: the attested taint comes 5s after the controller
// sees the attestation, the silent taint comes 10.55s after the last sign of
// life, and the heartbeat TTL is 4s.
const (
	attestedAfter = 5 * time.Second
	silentAfter   = 10550 * time.Millisecond
)

// TestEvictionOfASilentNodeWaitsForTheEndOfTheHeartbeats covers an armed node
// without an attestation. The node writes a heartbeat each second for one
// minute, and the controller sees each one. The node gets no taint while the
// heartbeats come. After the last heartbeat the node gets the taint when the
// silent delay passed after the last heartbeat plus heartbeat.interval.
func TestEvictionOfASilentNodeWaitsForTheEndOfTheHeartbeats(t *testing.T) {
	h := newHarness(t, failedState(), liveNode())
	h.reconcileAt(observedAt)

	last := observedAt

	for at := observedAt.Add(time.Second); !at.After(observedAt.Add(time.Minute)); at = at.Add(time.Second) {
		h.writeHeartbeat(at)
		h.reconcileAt(at)

		last = at
	}

	h.assertNotTainted()

	silentEnd := last.Add(time.Second + silentAfter)

	if res := h.reconcileAt(silentEnd.Add(-time.Millisecond)); res.RequeueAfter != time.Millisecond {
		t.Errorf("requeued after %s, want 1ms", res.RequeueAfter)
	}

	h.assertNotTainted()

	h.reconcileAt(silentEnd)
	h.assertTainted()
}

// TestEvictionStartsAtTheObservationOfTheAttestation writes an attestation that
// is one hour old. The deadline starts when the controller sees it.
func TestEvictionStartsAtTheObservationOfTheAttestation(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt.Add(-time.Hour))

	h := newHarness(t, incident, liveNode())

	if res := h.reconcileAt(observedAt); res.RequeueAfter != attestedAfter {
		t.Errorf("requeued after %s, want %s", res.RequeueAfter, attestedAfter)
	}

	h.reconcileAt(observedAt.Add(attestedAfter - time.Millisecond))
	h.assertNotTainted()

	if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
		t.Errorf("phase is %q before the deadline, want %q", got, v1alpha1.PhaseSuspected)
	}

	h.reconcileAt(observedAt.Add(attestedAfter))
	h.assertTainted()

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q for a node without pods", got, v1alpha1.PhaseEvicted)
	}
}

// TestEvictionOfANewLeaderOnlyDelays covers a change of the leader. The old
// leader saw the attestation 3s before the new leader. The new leader counts
// from its own observation.
func TestEvictionOfANewLeaderOnlyDelays(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	h.reconciler = New(h.client, h.api, agentNamespace, h.nodes, h.profiles, h.recorder)
	h.reconcileAt(observedAt.Add(3 * time.Second))

	h.reconcileAt(observedAt.Add(attestedAfter))
	h.assertNotTainted()

	h.reconcileAt(observedAt.Add(3*time.Second + attestedAfter))
	h.assertTainted()
}

// TestEvictionNeverTaintsAnUnarmedNode covers an unarmed node with an
// attestation, and an unarmed node whose object is in Proven.
func TestEvictionNeverTaintsAnUnarmedNode(t *testing.T) {
	for name, phase := range map[string]v1alpha1.FencingFailedNodeStatePhase{
		"attested":       v1alpha1.PhaseSuspected,
		"already proven": v1alpha1.PhaseProven,
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Spec.Armed = false
			incident.Spec.ProfileRef = nil
			incident.Status.Phase = phase
			attest(incident, observedAt)

			h := newHarness(t, incident, liveNode(), podOn("default", "web", toleration("", corev1.TolerationOpExists)))

			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(time.Hour))

			h.assertNotTainted()
			h.assertPods("default/web")

			if got := h.get().Status.Phase; got != phase {
				t.Errorf("phase is %q, want %q", got, phase)
			}
		})
	}
}

// TestEvictionStopsWhenTheNodeIsAliveAfterTheAttestation covers a heartbeat
// that the controller sees after the attestation. The attestation does not
// prove the failure. The silent delay starts again at the heartbeat.
func TestEvictionStopsWhenTheNodeIsAliveAfterTheAttestation(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	heartbeatAt := observedAt.Add(time.Second)
	h.writeHeartbeat(heartbeatAt)
	h.reconcileAt(heartbeatAt)

	silentEnd := heartbeatAt.Add(time.Second + silentAfter)

	for _, at := range []time.Time{observedAt.Add(attestedAfter), silentEnd.Add(-time.Millisecond)} {
		h.reconcileAt(at)
	}

	h.assertNotTainted()

	if got := testutil.ToFloat64(aliveAfterAttestationGauge.WithLabelValues(nodeName)); got != 1 {
		t.Errorf("d8_fencing_alive_after_attestation is %v, want 1", got)
	}

	h.reconcileAt(silentEnd)
	h.assertTainted()
}

// TestEvictionIgnoresHeartbeatsBeforeTheAttestation covers a node that wrote
// heartbeats before the attestation. They do not stop the taint, and the metric
// has no series.
func TestEvictionIgnoresHeartbeatsBeforeTheAttestation(t *testing.T) {
	incident := failedState()
	heartbeat := metav1.NewMicroTime(observedAt)
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	h.writeHeartbeat(observedAt.Add(time.Second))
	h.reconcileAt(observedAt.Add(time.Second))

	h.attest(observedAt.Add(2 * time.Second))
	h.reconcileAt(observedAt.Add(2 * time.Second))
	h.reconcileAt(observedAt.Add(2*time.Second + attestedAfter))

	h.assertTainted()

	if got := testutil.CollectAndCount(aliveAfterAttestationGauge); got != 0 {
		t.Errorf("d8_fencing_alive_after_attestation has %d series, want none", got)
	}
}

// TestEvictionKeepsTheTaintAfterAHeartbeat covers a heartbeat that the
// controller sees after the taint. The metric shows that the node is alive
// after the attestation, but the taint stays and the eviction continues. The
// taint and the object stay until the Node is Ready with a new boot ID.
func TestEvictionKeepsTheTaintAfterAHeartbeat(t *testing.T) {
	for name, tc := range map[string]struct {
		objects []client.Object
		// beforeTaint prepares the reconcile that sets the taint.
		beforeTaint func(h *harness)
		phase       v1alpha1.FencingFailedNodeStatePhase
	}{
		"evicted": {phase: v1alpha1.PhaseEvicted},
		"evicting": {
			objects: []client.Object{volumeAttachment("csi-own", nodeName)},
			phase:   v1alpha1.PhaseEvicting,
		},
		// A witness writes to the object after the taint, so the write of
		// Evicting fails with a conflict.
		"proven with the taint": {
			beforeTaint: func(h *harness) {
				patches := 0
				h.beforeStatusPatch = func() {
					if patches++; patches == 2 {
						h.beforeStatusPatch = nil
						h.writeContact(observedAt.Add(attestedAfter))
					}
				}
			},
			phase: v1alpha1.PhaseProven,
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			h := newHarness(t, append(tc.objects, incident, liveNode())...)
			h.reconcileAt(observedAt)

			if tc.beforeTaint != nil {
				tc.beforeTaint(h)
			}

			h.reconcileAt(observedAt.Add(attestedAfter))
			h.assertTainted()

			if got := h.get().Status.Phase; got != tc.phase {
				t.Fatalf("phase is %q after the taint, want %q", got, tc.phase)
			}

			heartbeatAt := observedAt.Add(attestedAfter + time.Second)
			h.writeHeartbeat(heartbeatAt)
			h.reconcileAt(heartbeatAt)

			if got := testutil.ToFloat64(aliveAfterAttestationGauge.WithLabelValues(nodeName)); got != 1 {
				t.Errorf("d8_fencing_alive_after_attestation is %v after the heartbeat, want 1", got)
			}

			h.assertTainted()

			if got := h.get().Status.Phase; got == v1alpha1.PhaseSuspected || got == v1alpha1.PhaseFallbackAlive {
				t.Errorf("phase is %q after the heartbeat, want the eviction to continue", got)
			}

			h.updateNode(func(n *corev1.Node) {
				n.Status.NodeInfo.BootID = returnedNode().Status.NodeInfo.BootID
				n.Status.Conditions = returnedNode().Status.Conditions
			})
			h.reconcileAt(heartbeatAt.Add(time.Second))

			h.assertIncidentGone()

			if h.tainted() {
				t.Errorf("node has the out-of-service taint after the return")
			}
		})
	}
}

// TestEvictionOfANewLeaderAfterTheTaint covers a new leader that sees the
// heartbeat and the attestation for the first time in one reconcile, after the
// taint. The Node gets no series of d8_fencing_alive_after_attestation, and
// the eviction continues.
func TestEvictionOfANewLeaderAfterTheTaint(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)
	heartbeat := metav1.NewMicroTime(observedAt.Add(-time.Second))
	incident.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}
	incident.Status.Phase = v1alpha1.PhaseEvicting

	node := liveNode()
	node.Spec.Taints = []corev1.Taint{outOfServiceTaint}

	h := newHarness(t, incident, node, volumeAttachment("csi-own", nodeName))
	h.reconcileAt(observedAt.Add(time.Minute))

	if got := testutil.CollectAndCount(aliveAfterAttestationGauge); got != 0 {
		t.Errorf("d8_fencing_alive_after_attestation has %d series after the taint, want none", got)
	}

	h.assertTainted()

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicting {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicting)
	}
}

// TestEvictionDecidesAgainAfterAConflict covers a heartbeat that the node
// writes between the read and the write of the controller. The write of Proven
// fails with a conflict, the Node gets no taint, and the next reconcile sees
// the heartbeat. The conflict is not an error: the controller only asks for a
// reconcile soon.
func TestEvictionDecidesAgainAfterAConflict(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	h.beforeStatusPatch = func() {
		h.beforeStatusPatch = nil
		h.writeHeartbeat(observedAt.Add(attestedAfter))
	}

	h.reconciler.now = h.clock(observedAt.Add(attestedAfter))

	res, err := h.reconcile()
	if err != nil || res.RequeueAfter <= 0 || res.RequeueAfter >= recheckAfter {
		t.Fatalf("reconcile returned %+v, %v, want a requeue sooner than %s and no error", res, err, recheckAfter)
	}

	h.assertNotTainted()

	h.reconcileAt(observedAt.Add(attestedAfter))
	h.reconcileAt(observedAt.Add(attestedAfter + time.Second + silentAfter - time.Millisecond))

	h.assertNotTainted()

	if got := h.get().Status.Phase; got == v1alpha1.PhaseProven {
		t.Errorf("phase is %q after the heartbeat", got)
	}
}

// TestEvictionRejectsAHeartbeatAfterProven covers the other order: the
// controller wrote Proven first. The heartbeat of the node, written on the
// version that it read before, fails with a conflict.
func TestEvictionRejectsAHeartbeatAfterProven(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	read := h.get()

	h.reconcileAt(observedAt.Add(attestedAfter))

	heartbeat := metav1.NewMicroTime(observedAt.Add(attestedAfter))
	read.Status.Fallback = &v1alpha1.FencingFailedNodeStateFallback{LastHeartbeatAt: &heartbeat}

	if err := h.api.Status().Update(t.Context(), read); !apierrors.IsConflict(err) {
		t.Fatalf("heartbeat on the old version returned %v, want a conflict", err)
	}
}

// TestEvictionWaitsForTheExclusions covers the maintenance annotations that
// prevent the taint of a proven incident. The controller sets the taint when
// the annotation is gone.
func TestEvictionWaitsForTheExclusions(t *testing.T) {
	for name, tc := range map[string]struct {
		node    func(*corev1.Node)
		release func(h *harness)
	}{
		"fencing-disable annotation": {
			node:    annotate("node-manager.deckhouse.io/fencing-disable"),
			release: func(h *harness) { h.updateNode(func(n *corev1.Node) { n.Annotations = nil }) },
		},
		"approved update": {
			node:    annotate("update.node.deckhouse.io/approved"),
			release: func(h *harness) { h.updateNode(func(n *corev1.Node) { n.Annotations = nil }) },
		},
		"approved disruption": {
			node:    annotate("update.node.deckhouse.io/disruption-approved"),
			release: func(h *harness) { h.updateNode(func(n *corev1.Node) { n.Annotations = nil }) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			tc.node(node)

			h := newHarness(t, incident, node)
			h.reconcileAt(observedAt)

			res := h.reconcileAt(observedAt.Add(attestedAfter))
			h.assertNotTainted()

			if got := h.get().Status.Phase; got != v1alpha1.PhaseProven {
				t.Errorf("phase is %q, want %q while the taint waits", got, v1alpha1.PhaseProven)
			}

			if res.RequeueAfter != recheckAfter {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
			}

			tc.release(h)
			h.reconcileAt(observedAt.Add(attestedAfter + recheckAfter))
			h.assertTainted()
		})
	}
}

// TestEvictionWaitsForTheArmedAgent covers a Node whose condition Fencing is
// not True: the agent is not armed now, for example in a rollout of the agents.
// The proof is complete, by the attestation or by the silent delay, but the
// controller does not write Proven and does not set the taint. The incident
// stays Suspected, and the controller examines the Node again. When the
// condition is True, the incident goes the usual way.
func TestEvictionWaitsForTheArmedAgent(t *testing.T) {
	proofs := map[string]struct {
		attested bool
		after    time.Duration
	}{
		"attested": {attested: true, after: attestedAfter},
		"silent":   {after: silentAfter},
	}

	conditions := map[string]func(*corev1.Node){
		"agent is stopping": withFencing(corev1.ConditionFalse, "AgentStopping"),
		"agent waits":       withFencing(corev1.ConditionFalse, "WaitingForControlPlane"),
		"status unknown":    withFencing(corev1.ConditionUnknown, "Armed"),
		"no condition":      func(n *corev1.Node) { n.Status.Conditions = nil },
	}

	for proofName, proof := range proofs {
		for conditionName, notArmed := range conditions {
			t.Run(proofName+", "+conditionName, func(t *testing.T) {
				incident := failedState()
				if proof.attested {
					attest(incident, observedAt)
				}

				node := liveNode()
				notArmed(node)

				h := newHarness(t, incident, node)
				h.reconcileAt(observedAt)

				res := h.reconcileAt(observedAt.Add(proof.after))
				h.assertNotTainted()

				if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
					t.Errorf("phase is %q, want %q while the agent is not armed", got, v1alpha1.PhaseSuspected)
				}

				if res.RequeueAfter != recheckAfter {
					t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
				}

				h.reconcileAt(observedAt.Add(time.Hour))
				h.assertNotTainted()

				h.updateNode(withFencing(corev1.ConditionTrue, "Armed"))
				h.reconcileAt(observedAt.Add(time.Hour + recheckAfter))
				h.assertTainted()
			})
		}
	}
}

// TestEvictionOfASilentNodeByANewLeader covers a change of the leader for a
// node that is off: no heartbeat and no attestation. The old leader saw the
// object 8s before the new leader, and it can stop after the write of Proven,
// before the taint. The new leader sees the object first in its first
// reconcile. It sets the taint not earlier than the silent delay after this
// reconcile.
func TestEvictionOfASilentNodeByANewLeader(t *testing.T) {
	for _, phase := range []v1alpha1.FencingFailedNodeStatePhase{v1alpha1.PhaseSuspected, v1alpha1.PhaseProven} {
		t.Run(string(phase), func(t *testing.T) {
			h := newHarness(t, failedState(), liveNode())
			h.reconcileAt(observedAt)

			if phase == v1alpha1.PhaseProven {
				incident := h.get()
				incident.Status.Phase = phase

				if err := h.api.Status().Update(t.Context(), incident); err != nil {
					t.Fatalf("write Proven of the old leader: %v", err)
				}
			}

			takeover := observedAt.Add(8 * time.Second)
			h.reconciler = New(h.client, h.api, agentNamespace, h.nodes, h.profiles, h.recorder)

			if res := h.reconcileAt(takeover); res.RequeueAfter != silentAfter {
				t.Errorf("the new leader requeued after %s, want %s", res.RequeueAfter, silentAfter)
			}

			for _, at := range []time.Time{observedAt.Add(silentAfter), takeover.Add(silentAfter - time.Millisecond)} {
				h.reconcileAt(at)
				h.assertNotTainted()
			}

			h.reconcileAt(takeover.Add(silentAfter))
			h.assertTainted()
		})
	}
}

// TestEvictionDoesNotTaintALiveNodeInARolloutOfTheAgent covers a live node
// that has an incident, for example after it lost the gossip. The node writes
// heartbeats. The agent stops for a rollout, so the heartbeats stop and the
// condition Fencing is not True. The silent delay passes, but the node gets no
// taint. The new agent writes a heartbeat and arms. Then the node is silent
// again, and it gets the taint after the silent delay.
func TestEvictionDoesNotTaintALiveNodeInARolloutOfTheAgent(t *testing.T) {
	h := newHarness(t, failedState(), liveNode())
	h.reconcileAt(observedAt)

	last := observedAt

	for at := observedAt.Add(time.Second); !at.After(observedAt.Add(5 * time.Second)); at = at.Add(time.Second) {
		h.writeHeartbeat(at)
		h.reconcileAt(at)

		last = at
	}

	// The old agent stops.
	h.updateNode(withFencing(corev1.ConditionFalse, "AgentStopping"))
	h.reconcileAt(last.Add(time.Second))

	// The new agent starts and waits before the arm.
	h.updateNode(withFencing(corev1.ConditionFalse, "WaitingForControlPlane"))

	for _, at := range []time.Time{last.Add(time.Second + silentAfter), last.Add(time.Minute)} {
		h.reconcileAt(at)
		h.assertNotTainted()

		if got := h.get().Status.Phase; got != v1alpha1.PhaseSuspected {
			t.Errorf("phase is %q at %s, want %q in the rollout", got, at.Sub(observedAt), v1alpha1.PhaseSuspected)
		}
	}

	// The new agent writes a heartbeat and arms.
	back := last.Add(time.Minute + time.Second)
	h.writeHeartbeat(back)
	h.updateNode(withFencing(corev1.ConditionTrue, "Armed"))
	h.reconcileAt(back)

	if got := h.get().Status.Phase; got != v1alpha1.PhaseFallbackAlive {
		t.Errorf("phase is %q after the new heartbeat, want %q", got, v1alpha1.PhaseFallbackAlive)
	}

	h.reconcileAt(back.Add(time.Second + silentAfter - time.Millisecond))
	h.assertNotTainted()

	h.reconcileAt(back.Add(time.Second + silentAfter))
	h.assertTainted()
}

// TestEvictionReadsTheNodeOnce covers the reconcile that sets the taint: it
// reads the Node from the API server one time before the taint.
func TestEvictionReadsTheNodeOnce(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	reads := 0
	h.reconciler.reader = interceptor.NewClient(h.reader.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Node); ok && h.nodePatches == 0 {
				reads++
			}

			return c.Get(ctx, key, obj, opts...)
		},
	})

	h.reconcileAt(observedAt.Add(attestedAfter))
	h.assertTainted()

	if reads != 1 {
		t.Errorf("the reconcile read the Node %d times before the taint, want 1", reads)
	}
}

// TestEvictionSeesAnExclusionAfterTheReadOfTheNode covers the agent that
// writes AgentStopping after the reconcile read the Node. The patch of the
// taint gets a conflict, the controller reads the Node again and waits.
func TestEvictionSeesAnExclusionAfterTheReadOfTheNode(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	// The write of Proven comes after the read of the Node and before the
	// patch of the taint.
	h.beforeStatusPatch = func() {
		h.beforeStatusPatch = nil
		h.updateNode(func(n *corev1.Node) {
			withFencing(corev1.ConditionFalse, "AgentStopping")(n)
		})
	}

	res := h.reconcileAt(observedAt.Add(attestedAfter))

	if h.tainted() {
		t.Errorf("node has the out-of-service taint, want none while the agent stops")
	}

	if res.RequeueAfter != recheckAfter {
		t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
	}
}

// TestEvictionDoesNotWaitForTheAgentPod covers a pod of the agent that is not
// ready or that is terminating, as in a rollout of the DaemonSet of the agents.
// The state of the pod of the agent does not prevent the taint.
func TestEvictionDoesNotWaitForTheAgentPod(t *testing.T) {
	for name, change := range map[string]func(*corev1.Pod){
		"not ready": func(p *corev1.Pod) {
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}}
		},
		"terminating": func(p *corev1.Pod) {
			p.Finalizers = []string{"test/keep"}
			p.DeletionTimestamp = &metav1.Time{Time: observedAt}
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			agent := agentPod()
			change(agent)

			h := newHarness(t, incident, liveNode(), agent)
			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(attestedAfter))

			h.assertTainted()
		})
	}
}

// TestEvictionKeepsTheTaintsOfOtherWriters checks that the controller adds its
// taint to the taints of the Node.
func TestEvictionKeepsTheTaintsOfOtherWriters(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	node := liveNode()
	node.Spec.Taints = []corev1.Taint{{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoExecute}}

	h := newHarness(t, incident, node)
	h.reconcileAt(observedAt)
	h.reconcileAt(observedAt.Add(attestedAfter))

	got := h.node().Spec.Taints
	if len(got) != 2 || got[0].Key != corev1.TaintNodeUnreachable {
		t.Errorf("taints are %+v, want the unreachable taint and the out-of-service taint", got)
	}
}

// TestEvictionFailsAndDeletesThePodsOfTheNode puts seven pods on the Node. The
// controller marks the pods Failed with the condition DisruptionTarget and
// deletes them with no grace period, also the pods that tolerate the taint. It
// only deletes the pod that is already Succeeded. The pod of a DaemonSet and
// the mirror pod stay. The pods that stay because of a finalizer do not stop
// Evicted.
func TestEvictionFailsAndDeletesThePodsOfTheNode(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	daemon := running(podOn("kube-system", "cilium", toleration("", corev1.TolerationOpExists)))
	daemon.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "apps/v1", Kind: "DaemonSet", Name: "cilium", UID: "d5", Controller: &isController,
	}}

	mirror := running(podOn("kube-system", "kube-apiserver", toleration("", corev1.TolerationOpExists)))
	mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "hash"}

	succeeded := kept(podOn("default", "job-1"))
	succeeded.Status.Phase = corev1.PodSucceeded

	h := newHarness(t, incident, liveNode(),
		kept(running(podOn("default", "web"))),
		kept(running(podOn("d8-system", "deckhouse", toleration("", corev1.TolerationOpExists)))),
		running(podOn("kube-system", "ccm", toleration(corev1.TaintNodeOutOfService, corev1.TolerationOpExists))),
		running(podOn("default", "api", toleration(corev1.TaintNodeUnreachable, corev1.TolerationOpExists))),
		succeeded, daemon, mirror,
	)
	h.reconcileAt(observedAt)
	h.reconcileAt(observedAt.Add(attestedAfter))

	h.assertTainted()
	h.assertPods("default/web", "d8-system/deckhouse", "default/job-1", "kube-system/cilium", "kube-system/kube-apiserver")

	for _, name := range []string{"default/web", "d8-system/deckhouse"} {
		pod := h.pod(name)
		target := disruptionTarget(pod)

		if pod.Status.Phase != corev1.PodFailed || pod.DeletionTimestamp == nil ||
			target == nil || target.Status != corev1.ConditionTrue || target.Reason != disruptionReason {
			t.Errorf("pod %s has phase %q, deletion %v and DisruptionTarget %+v, want a deleted Failed pod with DisruptionTarget",
				name, pod.Status.Phase, pod.DeletionTimestamp, target)
		}
	}

	if pod := h.pod("default/job-1"); pod.Status.Phase != corev1.PodSucceeded || disruptionTarget(pod) != nil || pod.DeletionTimestamp == nil {
		t.Errorf("pod default/job-1 has phase %q and deletion %v, want a deleted Succeeded pod without DisruptionTarget",
			pod.Status.Phase, pod.DeletionTimestamp)
	}

	for _, name := range []string{"kube-system/cilium", "kube-system/kube-apiserver"} {
		if pod := h.pod(name); pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil || disruptionTarget(pod) != nil {
			t.Errorf("pod %s has phase %q and deletion %v, want a running pod", name, pod.Status.Phase, pod.DeletionTimestamp)
		}
	}

	if len(h.podDeletions) != 5 {
		t.Errorf("controller deleted pods %d times, want 5", len(h.podDeletions))
	}

	for _, opts := range h.podDeletions {
		if opts.GracePeriodSeconds == nil || *opts.GracePeriodSeconds != 0 || opts.Preconditions == nil || opts.Preconditions.UID == nil {
			t.Errorf("pod deletion has options %+v, want no grace period and the UID precondition", opts)
		}
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}
}

// TestEvictionWaitsForTheVolumeAttachments covers the volumes of the deleted
// pods. The incident is Evicted only when the Node has no VolumeAttachments.
// The VolumeAttachment of another Node does not count.
func TestEvictionWaitsForTheVolumeAttachments(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode(), podOn("default", "web"),
		volumeAttachment("csi-own", nodeName), volumeAttachment("csi-other", "worker-1"))
	h.reconcileAt(observedAt)

	res := h.reconcileAt(observedAt.Add(attestedAfter))

	h.assertPods()

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicting || res.RequeueAfter != recheckAfter {
		t.Errorf("phase is %q with requeue after %s, want %q after %s", got, res.RequeueAfter, v1alpha1.PhaseEvicting, recheckAfter)
	}

	// The attach-detach controller detaches the volume.
	if err := h.api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
		t.Fatalf("delete volumeattachment: %v", err)
	}

	h.reconcileAt(observedAt.Add(attestedAfter + recheckAfter))

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}
}

// TestEvictionCompletesAfterLateVolumeAttachments covers a VolumeAttachment
// that stays for 90s. After 60s the incident stays in Evicting with the
// condition Progressing False. The controller examines the Node again. When
// the VolumeAttachment is gone, the incident is Evicted, the controller
// deletes the Node of the cloud machine and removes the condition.
func TestEvictionCompletesAfterLateVolumeAttachments(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	node := liveNode()
	capiNode("dvp")(node)

	h := newHarness(t, incident, node, healthCheck("dvp", 4, 4),
		volumeAttachment("csi-own", nodeName), volumeAttachment("csi-other", "worker-1"))
	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)

	if res := h.reconcileAt(start.Add(evictionTimeout)); res.RequeueAfter != recheckAfter {
		t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
	}

	got := h.get()
	if got.Status.Phase != v1alpha1.PhaseEvicting {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseEvicting)
	}

	assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonVolumesAttached)
	assertNotProgressing(t, common.ReasonVolumesAttached)

	if progress := meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing); progress == nil ||
		!strings.Contains(progress.Message, "csi-own") || strings.Contains(progress.Message, "csi-other") {
		t.Errorf("condition Progressing is %+v, want the message with the VolumeAttachment of the node only", progress)
	}

	if !h.exists(&corev1.Node{}, "", nodeName) {
		t.Errorf("node is deleted while the volume is attached")
	}

	if err := h.api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
		t.Fatalf("delete volumeattachment: %v", err)
	}

	h.reconcileAt(start.Add(90 * time.Second))

	got = h.get()
	if got.Status.Phase != v1alpha1.PhaseEvicted || meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing) != nil {
		t.Errorf("phase is %q with conditions %+v, want %q without Progressing", got.Status.Phase, got.Status.Conditions, v1alpha1.PhaseEvicted)
	}

	assertNotProgressing(t, "")

	if h.exists(&corev1.Node{}, "", nodeName) {
		t.Errorf("node is not deleted")
	}
}

// TestEvictionProgressingTellsTheCauseOfNow covers a cause that changes. The
// reason of the condition Progressing follows the state of the Node: first the
// VolumeAttachment, then the MachineHealthCheck that does not replace the
// machine. Each new cause gets a warning.
func TestEvictionProgressingTellsTheCauseOfNow(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	node := liveNode()
	capiNode("dvp")(node)

	h := newHarness(t, incident, node, healthCheck("dvp", 4, 1), volumeAttachment("csi-own", nodeName))
	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)
	h.reconcileAt(start.Add(evictionTimeout))
	h.reconcileAt(start.Add(evictionTimeout + recheckAfter))

	assertCondition(t, h.get(), common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonVolumesAttached)

	if err := h.api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
		t.Fatalf("delete volumeattachment: %v", err)
	}

	h.reconcileAt(start.Add(90 * time.Second))

	got := h.get()
	if got.Status.Phase != v1alpha1.PhaseEvicting {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseEvicting)
	}

	assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonTooManyUnhealthyMachines)
	assertNotProgressing(t, common.ReasonTooManyUnhealthyMachines)

	events := h.events()
	if got := countEvents(events, common.ReasonEvictionNotProgressing); got != 4 {
		t.Errorf("events are %q, want one warning for each cause on the incident and on the node", events)
	}
}

// TestEvictionReportsThePodsThatItCannotDelete covers an API that refuses the
// deletion of a pod. After 60s the reason is PodsRemaining with the pod and the
// error. When the API accepts the deletion, the incident is Evicted.
func TestEvictionReportsThePodsThatItCannotDelete(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode(), running(podOn("default", "web")))
	h.podDeleteErr = apierrors.NewInternalError(errors.New("etcdserver: request timed out"))
	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)
	h.reconcileAt(start.Add(evictionTimeout))

	got := h.get()
	assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonPodsRemaining)

	if progress := meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing); progress == nil ||
		!strings.Contains(progress.Message, "default/web") || !strings.Contains(progress.Message, "request timed out") {
		t.Errorf("condition Progressing is %+v, want the pod and the error in the message", progress)
	}

	h.podDeleteErr = nil
	h.reconcileAt(start.Add(evictionTimeout + recheckAfter))

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}

	h.assertPods()
}

// TestEvictionReportsAnotherError covers an error that is not about the pods,
// the volumes or the machines. After 60s the reason is PodsRemaining with the
// error. The controller examines the Node again after 5s.
func TestEvictionReportsAnotherError(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.attachmentsErr = apierrors.NewForbidden(schema.GroupResource{Group: "storage.k8s.io", Resource: "volumeattachments"}, "",
		errors.New("no access"))
	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)

	// The controller examines the Node again after 5s, not with the backoff
	// of an error.
	if res := h.reconcileAt(start.Add(evictionTimeout)); res.RequeueAfter != recheckAfter {
		t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
	}

	got := h.get()
	assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonPodsRemaining)

	if progress := meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing); progress == nil ||
		!strings.Contains(progress.Message, "no access") {
		t.Errorf("condition Progressing is %+v, want the error in the message", progress)
	}

	h.attachmentsErr = nil
	h.reconcileAt(start.Add(evictionTimeout + recheckAfter))

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}
}

// TestEvictionDeletesAPodOnce covers the pods that already have a deletion.
// The controller does not send a deletion again to a pod that has a deletion
// with no grace period. A pod with a grace period, for example from the taint,
// waits for the dead kubelet, so the controller deletes it with no grace
// period.
func TestEvictionDeletesAPodOnce(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	deleting := func(name string, grace int64) *corev1.Pod {
		pod := kept(podOn("default", name))
		pod.Status.Phase = corev1.PodFailed
		pod.DeletionTimestamp = &metav1.Time{Time: observedAt}
		pod.DeletionGracePeriodSeconds = &grace

		return pod
	}

	h := newHarness(t, incident, liveNode(), deleting("forced", 0), deleting("graceful", 30))
	h.reconcileAt(observedAt)
	h.reconcileAt(observedAt.Add(attestedAfter))

	if len(h.podDeletions) != 1 {
		t.Errorf("controller deleted pods %d times, want once for the pod with a grace period", len(h.podDeletions))
	}
}

// TestEvictionDeletesThePodsAtTheSameTime covers a Node with several pods. Each
// deletion waits until all deletions started, so the deletion of one pod after
// the other fails. The controller leaves the pods of a DaemonSet and the
// mirror pods, and the condition tells the failure of each pod that it cannot
// delete.
func TestEvictionDeletesThePodsAtTheSameTime(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	daemon := running(podOn("kube-system", "cilium", toleration("", corev1.TolerationOpExists)))
	daemon.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "apps/v1", Kind: "DaemonSet", Name: "cilium", UID: "d5", Controller: &isController,
	}}

	mirror := running(podOn("kube-system", "kube-apiserver", toleration("", corev1.TolerationOpExists)))
	mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "hash"}

	pods := []string{"web-0", "web-1", "web-2"}
	objects := []client.Object{incident, liveNode(), daemon, mirror}

	for _, name := range pods {
		objects = append(objects, running(podOn("default", name)))
	}

	h := newHarness(t, objects...)
	h.podDeleteErr = apierrors.NewInternalError(errors.New("etcdserver: request timed out"))

	var started atomic.Int32

	all := make(chan struct{})

	h.beforePodDelete = func() {
		if started.Add(1) == int32(len(pods)) {
			close(all)
		}

		select {
		case <-all:
		case <-time.After(time.Second):
			t.Errorf("pod deletion waited 1s for the deletions of the other pods, want all deletions at the same time")
		}
	}

	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)
	h.reconcileAt(start.Add(evictionTimeout))

	got := h.get()
	assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonPodsRemaining)

	message := meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing).Message
	for _, name := range pods {
		if !strings.Contains(message, "default/"+name) {
			t.Errorf("condition Progressing has the message %q, want the failure of default/%s", message, name)
		}
	}

	h.podDeleteErr = nil
	h.reconcileAt(start.Add(evictionTimeout + recheckAfter))

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}

	h.assertPods("kube-system/cilium", "kube-system/kube-apiserver")

	for _, name := range []string{"kube-system/cilium", "kube-system/kube-apiserver"} {
		if pod := h.pod(name); pod.Status.Phase != corev1.PodRunning || pod.DeletionTimestamp != nil || disruptionTarget(pod) != nil {
			t.Errorf("pod %s has phase %q and deletion %v, want a running pod", name, pod.Status.Phase, pod.DeletionTimestamp)
		}
	}
}

// TestEvictionChecksTheProofBeforeTheTaint covers a proven incident whose taint
// waits for an exclusion. The node writes a heartbeat while it waits. When the
// exclusion is gone, the node is alive, so it gets no taint before the silent
// delay after the heartbeat.
func TestEvictionChecksTheProofBeforeTheTaint(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	node := liveNode()
	annotate("node-manager.deckhouse.io/fencing-disable")(node)

	h := newHarness(t, incident, node)
	h.reconcileAt(observedAt)
	h.reconcileAt(observedAt.Add(attestedAfter))

	if got := h.get().Status.Phase; got != v1alpha1.PhaseProven {
		t.Fatalf("phase is %q, want %q while the taint waits", got, v1alpha1.PhaseProven)
	}

	heartbeatAt := observedAt.Add(attestedAfter + time.Second)
	h.writeHeartbeat(heartbeatAt)
	h.reconcileAt(heartbeatAt)

	h.updateNode(func(n *corev1.Node) { n.Annotations = nil })
	h.reconcileAt(observedAt.Add(attestedAfter + recheckAfter))
	h.reconcileAt(heartbeatAt.Add(time.Second + silentAfter - time.Millisecond))

	h.assertNotTainted()
}

// TestEvictionOfANewLeaderWaitsForItsOwnProof covers a new leader that finds an
// object in Proven or in Evicting, and a Node without the taint. The new leader
// sets the taint and deletes pods only after its own proof.
func TestEvictionOfANewLeaderWaitsForItsOwnProof(t *testing.T) {
	for _, phase := range []v1alpha1.FencingFailedNodeStatePhase{v1alpha1.PhaseProven, v1alpha1.PhaseEvicting} {
		t.Run(string(phase), func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			attest(incident, observedAt.Add(-time.Hour))

			h := newHarness(t, incident, liveNode(), podOn("d8-system", "deckhouse", toleration("", corev1.TolerationOpExists)))

			if res := h.reconcileAt(observedAt); res.RequeueAfter != attestedAfter {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, attestedAfter)
			}

			h.reconcileAt(observedAt.Add(attestedAfter - time.Millisecond))
			h.assertNotTainted()
			h.assertPods("d8-system/deckhouse")

			h.reconcileAt(observedAt.Add(attestedAfter))
			h.assertTainted()
			h.assertPods()
		})

		t.Run(string(phase)+" and a heartbeat", func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = phase
			attest(incident, observedAt.Add(-time.Hour))

			h := newHarness(t, incident, liveNode(), podOn("d8-system", "deckhouse", toleration("", corev1.TolerationOpExists)))
			h.reconcileAt(observedAt)

			h.writeHeartbeat(observedAt.Add(time.Second))
			h.reconcileAt(observedAt.Add(time.Second))
			h.reconcileAt(observedAt.Add(2*time.Second + silentAfter - time.Millisecond))

			h.assertNotTainted()
			h.assertPods("d8-system/deckhouse")
		})
	}
}

// TestEvictionSendsAgainARequestThatHangs covers a member of etcd that stops.
// The API server holds the first read and the first write of the controller.
// The controller sends each of them again after requestTimeout. Thus the taint
// is late only by these attempts, and each event comes once.
func TestEvictionSendsAgainARequestThatHangs(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)
	h.events()

	var readHung, writeHung atomic.Bool

	h.reconciler.reader = interceptor.NewClient(h.reader.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if readHung.CompareAndSwap(false, true) {
				return hang(ctx)
			}

			return c.Get(ctx, key, obj, opts...)
		},
	})
	h.reconciler.client = interceptor.NewClient(h.client.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			c client.Client,
			subResource string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if writeHung.CompareAndSwap(false, true) {
				return hang(ctx)
			}

			return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
		},
	})

	start := time.Now()
	h.reconcileAt(observedAt.Add(attestedAfter))

	if took, limit := time.Since(start), 2*requestTimeout+time.Second; took > limit {
		t.Errorf("reconcile took %s, want at most %s", took, limit)
	}

	if !readHung.Load() || !writeHung.Load() {
		t.Fatalf("read hung %t, write hung %t, want both", readHung.Load(), writeHung.Load())
	}

	h.assertTainted()

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}

	// Each event goes to the object and to the Node: Proven, the taint,
	// Evicting and Evicted.
	events := h.events()
	if got := countEvents(events, common.ReasonPhaseChanged); got != 6 {
		t.Errorf("%d events of a phase change, want 6: %v", got, events)
	}

	if got := countEvents(events, common.ReasonOutOfServiceTaintSet); got != 2 {
		t.Errorf("%d events of the taint, want 2: %v", got, events)
	}
}

// TestEvictionSendsAgainAWriteThatIsDone covers a write that is complete on the
// server, but its answer does not come in requestTimeout. The next attempt
// fails with a conflict and changes nothing. The controller does not write the
// phase again, and the Node gets one taint.
func TestEvictionSendsAgainAWriteThatIsDone(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)
	h.events()

	var statusHung, taintHung atomic.Bool

	// The write changes a copy of the object: a request that timed out does
	// not get the answer of the server.
	h.reconciler.client = interceptor.NewClient(h.client.(client.WithWatch), interceptor.Funcs{
		Patch: func(ctx context.Context, c client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if !taintHung.CompareAndSwap(false, true) {
				return c.Patch(ctx, obj, patch, opts...)
			}

			if err := c.Patch(ctx, obj.DeepCopyObject().(client.Object), patch, opts...); err != nil {
				return err
			}

			return hang(ctx)
		},
		SubResourcePatch: func(
			ctx context.Context,
			c client.Client,
			subResource string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if !statusHung.CompareAndSwap(false, true) {
				return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
			}

			if err := c.SubResource(subResource).Patch(ctx, obj.DeepCopyObject().(client.Object), patch, opts...); err != nil {
				return err
			}

			return hang(ctx)
		},
	})

	h.reconciler.now = h.clock(observedAt.Add(attestedAfter))

	res, err := h.reconcile()
	if err != nil || res.RequeueAfter <= 0 || res.RequeueAfter >= recheckAfter {
		t.Fatalf("reconcile returned %+v, %v, want a requeue sooner than %s and no error", res, err, recheckAfter)
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseProven {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseProven)
	}

	start := time.Now()
	h.reconcileAt(observedAt.Add(attestedAfter + res.RequeueAfter))

	if took, limit := time.Since(start), requestTimeout+time.Second; took > limit {
		t.Errorf("reconcile took %s, want at most %s", took, limit)
	}

	if !taintHung.Load() {
		t.Fatalf("taint did not hang")
	}

	taints := 0

	for _, taint := range h.node().Spec.Taints {
		if taint.MatchTaint(&outOfServiceTaint) {
			taints++
		}
	}

	if taints != 1 {
		t.Errorf("node has %d out-of-service taints, want 1", taints)
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}

	events := h.events()
	for _, phase := range []v1alpha1.FencingFailedNodeStatePhase{v1alpha1.PhaseProven, v1alpha1.PhaseEvicting, v1alpha1.PhaseEvicted} {
		if got := countEvents(events, "to "+string(phase)+"."); got > 2 {
			t.Errorf("%d events of the change to %s, want at most 2: %v", got, phase, events)
		}
	}

	if got := countEvents(events, common.ReasonOutOfServiceTaintSet); got > 2 {
		t.Errorf("%d events of the taint, want at most 2: %v", got, events)
	}
}

// TestEvictionSendsAgainAPodDeletionThatHangs covers a member of etcd that
// stops during the eviction. The API server holds the first patch of a pod and
// the first deletion of a pod. The controller sends each of them again after
// requestTimeout, so the eviction is late only by these attempts.
func TestEvictionSendsAgainAPodDeletionThatHangs(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode(), running(podOn("default", "web")))
	h.reconcileAt(observedAt)

	var patchHung, deleteHung atomic.Bool

	h.reconciler.client = interceptor.NewClient(h.client.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			c client.Client,
			subResource string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if _, ok := obj.(*corev1.Pod); ok && patchHung.CompareAndSwap(false, true) {
				return hang(ctx)
			}

			return c.SubResource(subResource).Patch(ctx, obj, patch, opts...)
		},
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.Pod); ok && deleteHung.CompareAndSwap(false, true) {
				return hang(ctx)
			}

			return c.Delete(ctx, obj, opts...)
		},
	})

	start := time.Now()
	h.reconcileAt(observedAt.Add(attestedAfter))

	if took, limit := time.Since(start), 2*requestTimeout+time.Second; took > limit {
		t.Errorf("reconcile took %s, want at most %s", took, limit)
	}

	if !patchHung.Load() || !deleteHung.Load() {
		t.Fatalf("patch hung %t, deletion hung %t, want both", patchHung.Load(), deleteHung.Load())
	}

	h.assertPods()

	if len(h.podDeletions) != 1 {
		t.Errorf("pod deletions reached the API %d times, want 1", len(h.podDeletions))
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}
}

// TestEvictionSendsAgainAReadOfTheReplacementThatHangs covers a member of etcd
// that stops after the deletion of the pods. The API server holds the reads
// that find the machine of the Node. The controller sends each of them again
// after requestTimeout, so Evicted is late only by these attempts.
func TestEvictionSendsAgainAReadOfTheReplacementThatHangs(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	h := newHarness(t, incident, liveNode())
	h.reconcileAt(observedAt)

	var evacuated, nodeHung, instancesHung atomic.Bool

	// The controller lists the VolumeAttachments of the Node immediately
	// before it looks for the machine of the Node.
	h.reconciler.reader = interceptor.NewClient(h.reader.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Node); ok && evacuated.Load() && nodeHung.CompareAndSwap(false, true) {
				return hang(ctx)
			}

			return c.Get(ctx, key, obj, opts...)
		},
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			switch list.(type) {
			case *storagev1.VolumeAttachmentList:
				evacuated.Store(true)
			case *unstructured.UnstructuredList:
				if instancesHung.CompareAndSwap(false, true) {
					return hang(ctx)
				}
			}

			return c.List(ctx, list, opts...)
		},
	})

	start := time.Now()
	h.reconcileAt(observedAt.Add(attestedAfter))

	if took, limit := time.Since(start), 2*requestTimeout+time.Second; took > limit {
		t.Errorf("reconcile took %s, want at most %s", took, limit)
	}

	if !nodeHung.Load() || !instancesHung.Load() {
		t.Fatalf("node read hung %t, instance list hung %t, want both", nodeHung.Load(), instancesHung.Load())
	}

	if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
	}
}

// hang holds a request as the API server does when the request waits for a
// member of etcd that is down: until the end of the context, or 10s.
func hang(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return errors.New("read: connection timed out")
	}
}

// attest writes the attestation of a witness to the object.
func attest(incident *v1alpha1.FencingFailedNodeState, at time.Time) {
	incident.Status.Attestation = &v1alpha1.FencingFailedNodeStateAttestation{
		Witness: "worker-1",
		At:      metav1.NewMicroTime(at),
	}
}

// attest writes an attestation, as the witness does.
func (h *harness) attest(at time.Time) {
	h.t.Helper()

	incident := h.get()
	attest(incident, at)

	if err := h.api.Status().Update(h.t.Context(), incident); err != nil {
		h.t.Fatalf("write attestation: %v", err)
	}
}

// withFencing sets the condition Fencing of the Node, as its agent does.
func withFencing(status corev1.ConditionStatus, reason string) func(*corev1.Node) {
	return func(n *corev1.Node) {
		n.Status.Conditions = []corev1.NodeCondition{{Type: fencingConditionType, Status: status, Reason: reason}}
	}
}

func (h *harness) node() *corev1.Node {
	h.t.Helper()

	var node corev1.Node
	if err := h.api.Get(h.t.Context(), types.NamespacedName{Name: nodeName}, &node); err != nil {
		h.t.Fatalf("get node: %v", err)
	}

	return &node
}

func (h *harness) updateNode(change func(*corev1.Node)) {
	h.t.Helper()

	node := h.node()
	change(node)

	if err := h.api.Update(h.t.Context(), node); err != nil {
		h.t.Fatalf("update node: %v", err)
	}

	// The update keeps the old status, so the change goes to the status too.
	change(node)

	if err := h.api.Status().Update(h.t.Context(), node); err != nil {
		h.t.Fatalf("update node status: %v", err)
	}
}

func (h *harness) tainted() bool {
	h.t.Helper()

	for i := range h.node().Spec.Taints {
		if h.node().Spec.Taints[i].MatchTaint(&outOfServiceTaint) {
			return true
		}
	}

	return false
}

func (h *harness) assertTainted() {
	h.t.Helper()

	if !h.tainted() {
		h.t.Errorf("node has no out-of-service taint")
	}
}

func (h *harness) assertNotTainted() {
	h.t.Helper()

	if h.tainted() || h.nodePatches != 0 {
		h.t.Errorf("node was patched %d times and has taints %+v, want no taint", h.nodePatches, h.node().Spec.Taints)
	}
}

// assertPods checks the pods that are on the Node, as namespace/name.
func (h *harness) assertPods(want ...string) {
	h.t.Helper()

	var pods corev1.PodList
	if err := h.api.List(h.t.Context(), &pods, client.MatchingFields{podNodeNameField: nodeName}); err != nil {
		h.t.Fatalf("list pods: %v", err)
	}

	got := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		got = append(got, pod.Namespace+"/"+pod.Name)
	}

	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		h.t.Errorf("pods on the node are %v, want %v", got, want)
	}
}

func podOn(namespace, name string, tolerations ...corev1.Toleration) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, UID: types.UID(namespace + "-" + name)},
		Spec:       corev1.PodSpec{NodeName: nodeName, Tolerations: tolerations},
	}
}

// running sets the phase Running of the pod.
func running(pod *corev1.Pod) *corev1.Pod {
	pod.Status.Phase = corev1.PodRunning

	return pod
}

// kept gives the pod a finalizer, so the pod stays in the API after the deletion.
func kept(pod *corev1.Pod) *corev1.Pod {
	pod.Finalizers = append(pod.Finalizers, "test/keep")

	return pod
}

// pod returns the pod with the name namespace/name.
func (h *harness) pod(name string) *corev1.Pod {
	h.t.Helper()

	namespace, name, _ := strings.Cut(name, "/")

	var pod corev1.Pod
	if err := h.api.Get(h.t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		h.t.Fatalf("get pod: %v", err)
	}

	return &pod
}

func disruptionTarget(pod *corev1.Pod) *corev1.PodCondition {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == corev1.DisruptionTarget {
			return &pod.Status.Conditions[i]
		}
	}

	return nil
}

func volumeAttachment(name, node string) *storagev1.VolumeAttachment {
	return &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       storagev1.VolumeAttachmentSpec{NodeName: node, Attacher: "csi.test"},
	}
}

func toleration(key string, operator corev1.TolerationOperator) corev1.Toleration {
	return corev1.Toleration{Key: key, Operator: operator}
}

func annotate(key string) func(*corev1.Node) {
	return func(n *corev1.Node) { n.Annotations = map[string]string{key: ""} }
}

// agentPod is the pod of the fencing agent on the Node.
func agentPod() *corev1.Pod {
	pod := podOn(agentNamespace, "fencing-agent-x", toleration("", corev1.TolerationOpExists))
	pod.Labels = map[string]string{"app": "fencing-agent"}
	pod.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "apps/v1", Kind: "DaemonSet", Name: "fencing-agent-worker", UID: "a9", Controller: &isController,
	}}

	return pod
}

// isController marks the owner reference of a controller.
var isController = true

func microTime(at time.Time) *metav1.MicroTime {
	t := metav1.NewMicroTime(at)

	return &t
}
