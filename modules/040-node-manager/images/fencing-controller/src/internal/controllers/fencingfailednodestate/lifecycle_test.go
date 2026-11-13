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
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "fencing-controller/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-controller/internal/common"
)

// TestEvictionDoesNotWaitForOtherEvictions covers several failed Nodes at the
// same time. Another Node is in Evicting, and the controller processes two
// proven incidents at the same time. Each Node gets the taint at its deadline.
func TestEvictionDoesNotWaitForOtherEvictions(t *testing.T) {
	resetMetrics()

	names := []string{nodeName, "worker-5"}
	objects := []client.Object{otherIncident("worker-1", v1alpha1.PhaseEvicting)}

	for _, name := range names {
		node := liveNode()
		node.Name = name
		node.UID = types.UID("node-" + name)

		incident := failedState()
		incident.Name = name
		incident.UID = types.UID("incident-" + name)
		incident.OwnerReferences[0].Name = name
		incident.OwnerReferences[0].UID = node.UID
		attest(incident, observedAt)

		// The volume keeps the Node that gets the taint in eviction.
		objects = append(objects, node, incident, volumeAttachment("csi-"+name, name))
	}

	reconciler, api := newRealReconciler(t, objects...)

	for _, name := range names {
		reconcileOnce(t, reconciler, api, name, observedAt)
	}

	// The deadline of the critical profile is 5.3s.
	reconciler.now = func() time.Time { return observedAt.Add(5300 * time.Millisecond) }

	var reconciles sync.WaitGroup

	for _, name := range names {
		reconciles.Go(func() {
			if _, err := reconciler.Reconcile(t.Context(), request(name)); err != nil {
				t.Errorf("reconcile %s: %v", name, err)
			}
		})
	}

	reconciles.Wait()

	for _, name := range names {
		var node corev1.Node
		if err := api.Get(t.Context(), types.NamespacedName{Name: name}, &node); err != nil {
			t.Fatalf("get node %s: %v", name, err)
		}

		if !slices.ContainsFunc(node.Spec.Taints, func(taint corev1.Taint) bool { return taint.MatchTaint(&outOfServiceTaint) }) {
			t.Errorf("node %s has no taint at the deadline", name)
		}

		var incident v1alpha1.FencingFailedNodeState
		if err := api.Get(t.Context(), types.NamespacedName{Name: name}, &incident); err != nil {
			t.Fatalf("get fencingfailednodestate %s: %v", name, err)
		}

		if incident.Status.Phase != v1alpha1.PhaseEvicting {
			t.Errorf("phase of %s is %q, want %q", name, incident.Status.Phase, v1alpha1.PhaseEvicting)
		}
	}
}

// TestReturnedNodeIsReleased covers a Node that came back after a reboot: it is
// Ready and it has a new boot ID. The controller removes the out-of-service
// taint and deletes the object. The rule is the same for each incident.
func TestReturnedNodeIsReleased(t *testing.T) {
	for name, change := range map[string]func(*v1alpha1.FencingFailedNodeState){
		"evicted":   func(i *v1alpha1.FencingFailedNodeState) { i.Status.Phase = v1alpha1.PhaseEvicted },
		"evicting":  func(i *v1alpha1.FencingFailedNodeState) { i.Status.Phase = v1alpha1.PhaseEvicting },
		"suspected": func(i *v1alpha1.FencingFailedNodeState) { i.Status.Phase = v1alpha1.PhaseSuspected },
		"unarmed": func(i *v1alpha1.FencingFailedNodeState) {
			i.Spec.Armed = false
			i.Spec.ProfileRef = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			change(incident)

			node := returnedNode()
			node.Spec.Taints = []corev1.Taint{
				{Key: corev1.TaintNodeUnreachable, Effect: corev1.TaintEffectNoExecute},
				outOfServiceTaint,
			}

			h := newHarness(t, incident, node)
			h.reconcileAt(observedAt.Add(time.Second))

			if taints := h.node().Spec.Taints; len(taints) != 1 || taints[0].Key != corev1.TaintNodeUnreachable {
				t.Errorf("taints are %+v, want only the unreachable taint", taints)
			}

			h.assertIncidentGone()

			if got := countEvents(h.events(), common.ReasonOutOfServiceTaintRemoved); got != 2 {
				t.Errorf("published %d events of the removed taint, want 2: one on the object and one on the node", got)
			}

			if got := testutil.CollectAndCount(phaseGauge); got != 0 {
				t.Errorf("phase metric has %d series after the object is deleted, want none", got)
			}
		})
	}
}

// TestReturnedNodeWithoutTheTaint covers an unarmed node that never got the
// taint. The controller deletes the object and publishes no event of a removed
// taint.
func TestReturnedNodeWithoutTheTaint(t *testing.T) {
	incident := failedState()
	incident.Spec.Armed = false
	incident.Spec.ProfileRef = nil

	h := newHarness(t, incident, returnedNode())
	h.reconcileAt(observedAt.Add(time.Second))

	h.assertIncidentGone()

	if h.nodePatches != 0 {
		t.Errorf("node was patched %d times, want none", h.nodePatches)
	}

	if got := countEvents(h.events(), common.ReasonOutOfServiceTaintRemoved); got != 0 {
		t.Errorf("published %d events of the removed taint, want none", got)
	}
}

// TestReturnedNodeDoesNotWaitForTheAgent covers a Node that came back, but its
// fencing agent did not start. The agent can be absent, or its condition can
// stay from the boot before the failure. The controller removes the taint and
// deletes the object. The log of the return does not use the condition.
func TestReturnedNodeDoesNotWaitForTheAgent(t *testing.T) {
	for name, conditions := range map[string][]corev1.NodeCondition{
		"no fencing condition": nil,
		"fencing condition of the old boot": {{
			Type: fencingConditionType, Status: corev1.ConditionTrue, Reason: "Armed",
			LastTransitionTime: metav1.NewTime(observedAt.Add(-time.Hour)),
		}},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = v1alpha1.PhaseEvicted

			node := returnedNode()
			node.Spec.Taints = []corev1.Taint{outOfServiceTaint}
			node.Status.Conditions = append(node.Status.Conditions, conditions...)

			h := newHarness(t, incident, node)
			h.reconciler.now = h.clock(observedAt.Add(time.Second))

			var lines []string

			logger := funcr.New(func(_, args string) { lines = append(lines, args) }, funcr.Options{})

			if _, err := h.reconciler.Reconcile(logf.IntoContext(t.Context(), logger), request(nodeName)); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			if h.tainted() {
				t.Errorf("node has the out-of-service taint, want it removed")
			}

			h.assertIncidentGone()

			i := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "node came back after a reboot") })
			if i < 0 {
				t.Fatalf("no log of the return in %q", lines)
			}

			if !strings.Contains(lines[i], `"node_boot_id"="77776666-1111-2222-3333-444455556666"`) ||
				strings.Contains(lines[i], "fencing_condition") {
				t.Errorf("log of the return is %s, want the boot ID of the Node and no time of the Fencing condition", lines[i])
			}
		})
	}
}

// TestReturnedNodeNeedsAllConditions covers a Node that does not meet one of the
// conditions of the return. The taint stays, and the object stays.
func TestReturnedNodeNeedsAllConditions(t *testing.T) {
	for name, change := range map[string]func(*corev1.Node){
		"same boot ID": func(n *corev1.Node) { n.Status.NodeInfo.BootID = failedState().Spec.BootID },
		"not ready": func(n *corev1.Node) {
			n.Status.Conditions[0].Status = corev1.ConditionFalse
		},
		"ready unknown": func(n *corev1.Node) {
			n.Status.Conditions[0].Status = corev1.ConditionUnknown
		},
		"no boot ID":   func(n *corev1.Node) { n.Status.NodeInfo.BootID = "" },
		"no condition": func(n *corev1.Node) { n.Status.Conditions = nil },
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			incident.Status.Phase = v1alpha1.PhaseEvicted

			node := returnedNode()
			node.Spec.Taints = []corev1.Taint{outOfServiceTaint}
			change(node)

			h := newHarness(t, incident, node)
			h.reconcileAt(observedAt.Add(time.Second))

			h.assertTainted()

			if got := countEvents(h.events(), common.ReasonOutOfServiceTaintRemoved); got != 0 {
				t.Errorf("published %d events of the removed taint, want none", got)
			}

			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
			}
		})
	}
}

// TestEvictionReplacesTheMachine covers the Nodes that the cloud replaces. For
// a cloud CAPI machine the controller deletes the Node when at most half of
// the machines are unhealthy. For an MCM machine it deletes the Machine.
func TestEvictionReplacesTheMachine(t *testing.T) {
	for name, tc := range map[string]struct {
		node        func(*corev1.Node)
		objects     []client.Object
		nodeDeleted bool
	}{
		"capi, 1 of 4 unhealthy": {
			node:        capiNode("dvp"),
			objects:     []client.Object{healthCheck("dvp", 4, 3)},
			nodeDeleted: true,
		},
		"capi, 2 of 4 unhealthy": {
			node:        capiNode("dvp"),
			objects:     []client.Object{healthCheck("dvp", 4, 2)},
			nodeDeleted: true,
		},
		"capi from the instance": {
			node: func(n *corev1.Node) { n.Annotations = map[string]string{capiClusterNameAnnotation: "dvp"} },
			objects: []client.Object{
				instance(nodeName, "cluster.x-k8s.io/v1beta2", "dvp-worker-abcde"),
				healthCheck("dvp", 4, 3),
			},
			nodeDeleted: true,
		},
		"mcm": {
			objects: []client.Object{instance("instance-1", "machine.sapcloud.io/v1alpha1", nodeName), machine(nodeName)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			if tc.node != nil {
				tc.node(node)
			}

			h := newHarness(t, append(tc.objects, incident, node, machine("other"), volumeAttachment("csi-own", nodeName))...)
			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(attestedAfter))

			// The controller replaces the machine only after the volumes are
			// detached.
			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicting {
				t.Errorf("phase is %q while the volume is attached, want %q", got, v1alpha1.PhaseEvicting)
			}

			if !h.exists(&corev1.Node{}, "", nodeName) || (!tc.nodeDeleted && !h.exists(newMachine(), agentNamespace, nodeName)) {
				t.Errorf("node or machine is deleted while the volume is attached")
			}

			if err := h.api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
				t.Fatalf("delete volumeattachment: %v", err)
			}

			h.reconcileAt(observedAt.Add(attestedAfter + recheckAfter))

			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
			}

			if got := h.exists(&corev1.Node{}, "", nodeName); got == tc.nodeDeleted {
				t.Errorf("node exists: %t, want %t", got, !tc.nodeDeleted)
			}

			if !tc.nodeDeleted && h.exists(newMachine(), agentNamespace, nodeName) {
				t.Errorf("machine %s is not deleted", nodeName)
			}

			if !h.exists(newMachine(), agentNamespace, "other") {
				t.Errorf("machine of another node is deleted")
			}
		})
	}
}

// TestEvictionWritesEvictedBeforeTheReplacement covers the last step of the
// eviction. The controller writes Evicted first. It deletes the Node or the
// Machine only after this write. When another writer changes the object before
// the write, the write fails with a conflict, and the controller deletes
// nothing. When the deletion fails after the write, the controller sends it
// again for the Evicted incident.
func TestEvictionWritesEvictedBeforeTheReplacement(t *testing.T) {
	for name, tc := range map[string]struct {
		node    func(*corev1.Node)
		objects []client.Object
		// replaced tells if the object that gets a new machine is deleted.
		replaced func(h *harness) bool
	}{
		"capi": {
			node:     capiNode("dvp"),
			objects:  []client.Object{healthCheck("dvp", 4, 3)},
			replaced: func(h *harness) bool { return !h.exists(&corev1.Node{}, "", nodeName) },
		},
		"mcm": {
			objects:  []client.Object{instance("instance-1", "machine.sapcloud.io/v1alpha1", nodeName), machine(nodeName)},
			replaced: func(h *harness) bool { return !h.exists(newMachine(), agentNamespace, nodeName) },
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			if tc.node != nil {
				tc.node(node)
			}

			h := newHarness(t, append(tc.objects, incident, node, volumeAttachment("csi-own", nodeName))...)
			h.reconcileAt(observedAt)

			at := observedAt.Add(attestedAfter)
			h.reconcileAt(at)

			if err := h.api.Delete(t.Context(), volumeAttachment("csi-own", nodeName)); err != nil {
				t.Fatalf("delete volumeattachment: %v", err)
			}

			// A peer writes its contact with the node before the write of
			// Evicted.
			h.beforeStatusPatch = func() {
				h.beforeStatusPatch = nil
				h.writeContact(at)
			}

			at = at.Add(recheckAfter)
			h.reconciler.now = h.clock(at)

			if res, err := h.reconcile(); err != nil || res.RequeueAfter <= 0 || res.RequeueAfter >= recheckAfter {
				t.Fatalf("reconcile returned %+v, %v, want a requeue sooner than %s and no error after the conflict", res, err, recheckAfter)
			}

			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicting || tc.replaced(h) {
				t.Fatalf("phase is %q, replacement sent %t after the conflict, want %q and no deletion",
					got, tc.replaced(h), v1alpha1.PhaseEvicting)
			}

			replacedBeforeEvicted := false
			h.beforeStatusPatch = func() { replacedBeforeEvicted = tc.replaced(h) }
			h.replaceErr = apierrors.NewServiceUnavailable("test")

			at = at.Add(recheckAfter)
			h.reconciler.now = h.clock(at)

			if _, err := h.reconcile(); err == nil {
				t.Errorf("reconcile returned no error, want the error of the deletion")
			}

			if replacedBeforeEvicted {
				t.Errorf("replacement is sent before the write of Evicted")
			}

			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted || tc.replaced(h) {
				t.Fatalf("phase is %q, replacement sent %t after the failed deletion, want %q and no deletion",
					got, tc.replaced(h), v1alpha1.PhaseEvicted)
			}

			h.replaceErr = nil
			h.reconcileAt(at.Add(recheckAfter))

			if !tc.replaced(h) {
				t.Errorf("replacement is not sent again for the Evicted incident")
			}
		})
	}
}

// TestEvictionKeepsTheNodesThatAreNotReplaced covers the Nodes that keep only
// the taint. A master and an etcd arbiter are never deleted, also when they
// have a machine.
func TestEvictionKeepsTheNodesThatAreNotReplaced(t *testing.T) {
	label := func(key string) func(*corev1.Node) {
		return func(n *corev1.Node) {
			capiNode("dvp")(n)
			n.Labels = map[string]string{key: ""}
		}
	}

	for name, tc := range map[string]struct {
		node    func(*corev1.Node)
		objects []client.Object
	}{
		"static": {},
		"caps":   {node: capiNode(staticClusterName)},
		"capi without a cluster": {
			node: func(n *corev1.Node) { n.Annotations = map[string]string{capiMachineAnnotation: "dvp-worker-abcde"} },
		},
		"instance without a machine": {objects: []client.Object{instance(nodeName, "", "")}},
		"capi cluster without a machine": {
			node: func(n *corev1.Node) { n.Annotations = map[string]string{capiClusterNameAnnotation: "dvp"} },
		},
		"master":        {node: label("node-role.kubernetes.io/control-plane")},
		"legacy master": {node: label("node-role.kubernetes.io/master")},
		"etcd arbiter":  {node: label("node.deckhouse.io/etcd-arbiter")},
		"master with an mcm machine": {
			node:    func(n *corev1.Node) { n.Labels = map[string]string{"node-role.kubernetes.io/control-plane": ""} },
			objects: []client.Object{instance(nodeName, "machine.sapcloud.io/v1alpha1", nodeName)},
		},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			if tc.node != nil {
				tc.node(node)
			}

			h := newHarness(t, append(tc.objects, incident, node, healthCheck("dvp", 4, 4), healthCheck(staticClusterName, 4, 4),
				machine(nodeName))...)
			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(attestedAfter))

			h.assertTainted()

			if got := h.get().Status.Phase; got != v1alpha1.PhaseEvicted {
				t.Errorf("phase is %q, want %q", got, v1alpha1.PhaseEvicted)
			}

			if !h.exists(newMachine(), agentNamespace, nodeName) {
				t.Errorf("machine is deleted")
			}
		})
	}
}

// TestEvictionDoesNotProgressWhenTooManyMachinesAreUnhealthy covers a cloud
// CAPI machine that no MachineHealthCheck replaces now. The Node stays. After
// 60s the incident stays in Evicting with the condition Progressing False and
// the reason.
func TestEvictionDoesNotProgressWhenTooManyMachinesAreUnhealthy(t *testing.T) {
	for name, objects := range map[string][]client.Object{
		"3 of 4 unhealthy":      {healthCheck("dvp", 4, 1)},
		"3 of 4 in one of two":  {healthCheck("dvp", 4, 1), renamed(healthCheck("dvp", 4, 4), "dvp-custom")},
		"no machinehealthcheck": {healthCheck("other", 4, 4)},
		"no machines":           {healthCheck("dvp", 0, 0)},
		"no status":             {withoutStatus(healthCheck("dvp", 4, 4))},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			capiNode("dvp")(node)

			h := newHarness(t, append(objects, incident, node)...)
			h.reconcileAt(observedAt)

			start := observedAt.Add(attestedAfter)
			h.reconcileAt(start)

			if res := h.reconcileAt(start.Add(evictionTimeout - time.Second)); res.RequeueAfter != time.Second {
				t.Errorf("requeued after %s, want 1s until the timeout", res.RequeueAfter)
			}

			if got := h.get(); meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing) != nil {
				t.Errorf("incident has the condition Progressing before the timeout: %+v", got.Status.Conditions)
			}

			if res := h.reconcileAt(start.Add(evictionTimeout)); res.RequeueAfter != recheckAfter {
				t.Errorf("requeued after %s, want %s", res.RequeueAfter, recheckAfter)
			}

			got := h.get()
			if got.Status.Phase != v1alpha1.PhaseEvicting {
				t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseEvicting)
			}

			assertCondition(t, got, common.ConditionTypeProgressing, metav1.ConditionFalse, common.ReasonTooManyUnhealthyMachines)
			assertNotProgressing(t, common.ReasonTooManyUnhealthyMachines)

			if !h.exists(&corev1.Node{}, "", nodeName) {
				t.Errorf("node is deleted")
			}

			h.assertTainted()
		})
	}
}

// TestEvictionCountsAReadyNodeAsUnhealthy covers an eviction before the
// MachineHealthCheck sees the failure: the Node is still Ready, and the
// MachineHealthCheck counts it as healthy. The controller counts the machine
// as unhealthy itself.
func TestEvictionCountsAReadyNodeAsUnhealthy(t *testing.T) {
	for name, tc := range map[string]struct {
		ready corev1.ConditionStatus
		// healthy is currentHealthy of 4 machines in the MachineHealthCheck.
		healthy     int64
		nodeDeleted bool
	}{
		"ready node, 2 of 4 unhealthy with it": {ready: corev1.ConditionTrue, healthy: 3, nodeDeleted: true},
		"ready node, 3 of 4 unhealthy with it": {ready: corev1.ConditionTrue, healthy: 2},
		"node not ready, already counted":      {ready: corev1.ConditionUnknown, healthy: 2, nodeDeleted: true},
	} {
		t.Run(name, func(t *testing.T) {
			incident := failedState()
			attest(incident, observedAt)

			node := liveNode()
			capiNode("dvp")(node)
			node.Status.Conditions = append(node.Status.Conditions, corev1.NodeCondition{Type: corev1.NodeReady, Status: tc.ready})

			h := newHarness(t, incident, node, healthCheck("dvp", 4, tc.healthy))
			h.reconcileAt(observedAt)
			h.reconcileAt(observedAt.Add(attestedAfter))

			if got := h.exists(&corev1.Node{}, "", nodeName); got == tc.nodeDeleted {
				t.Errorf("node exists: %t, want %t", got, !tc.nodeDeleted)
			}
		})
	}
}

// TestEvictionDeletesTheNodeWhenTheMachinesRecover covers machines that become
// healthy after the timeout. The controller examines the Node again, deletes
// it and removes the condition Progressing.
func TestEvictionDeletesTheNodeWhenTheMachinesRecover(t *testing.T) {
	incident := failedState()
	attest(incident, observedAt)

	node := liveNode()
	capiNode("dvp")(node)

	h := newHarness(t, incident, node, healthCheck("dvp", 4, 1))
	h.reconcileAt(observedAt)

	start := observedAt.Add(attestedAfter)
	h.reconcileAt(start)
	h.reconcileAt(start.Add(evictionTimeout))
	assertNotProgressing(t, common.ReasonTooManyUnhealthyMachines)

	check := healthCheck("dvp", 4, 3)
	check.SetResourceVersion(h.resourceVersion(withoutStatus(healthCheck("dvp", 0, 0))))

	if err := h.api.Update(t.Context(), check); err != nil {
		t.Fatalf("update machinehealthcheck: %v", err)
	}

	h.reconcileAt(start.Add(90 * time.Second))

	got := h.get()
	if got.Status.Phase != v1alpha1.PhaseEvicted {
		t.Errorf("phase is %q, want %q", got.Status.Phase, v1alpha1.PhaseEvicted)
	}

	if meta.FindStatusCondition(got.Status.Conditions, common.ConditionTypeProgressing) != nil {
		t.Errorf("condition Progressing stays after the eviction: %+v", got.Status.Conditions)
	}

	assertNotProgressing(t, "")

	if h.exists(&corev1.Node{}, "", nodeName) {
		t.Errorf("node is not deleted")
	}
}

// assertNotProgressing checks the series of d8_fencing_eviction_not_progressing
// of the Node. An empty reason means no series.
func assertNotProgressing(t *testing.T, reason string) {
	t.Helper()

	want := 0
	if reason != "" {
		want = 1
	}

	if got := testutil.CollectAndCount(notProgressingGauge); got != want {
		t.Errorf("d8_fencing_eviction_not_progressing has %d series, want %d", got, want)
	}

	if reason != "" {
		if got := testutil.ToFloat64(notProgressingGauge.WithLabelValues(nodeName, reason)); got != 1 {
			t.Errorf("d8_fencing_eviction_not_progressing{reason=%q} is %v, want 1", reason, got)
		}
	}
}

// countEvents returns the number of the events that contain the text.
func countEvents(events []string, text string) int {
	count := 0

	for _, event := range events {
		if strings.Contains(event, text) {
			count++
		}
	}

	return count
}

// otherIncident is the object of another Node in the given phase.
func otherIncident(name string, phase v1alpha1.FencingFailedNodeStatePhase) *v1alpha1.FencingFailedNodeState {
	incident := failedState()
	incident.Name = name
	incident.UID = types.UID("uid-" + name)
	incident.Status.Phase = phase

	return incident
}

// returnedNode is the Node after a reboot: Ready, with a new boot ID, and
// without the condition of the fencing agent. The Ready condition is the first
// one.
func returnedNode() *corev1.Node {
	node := liveNode()
	node.Status.NodeInfo.BootID = "77776666-1111-2222-3333-444455556666"
	node.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}

	return node
}

// capiNode gives the Node the annotations that CAPI sets on the Node of a
// machine of the cluster.
func capiNode(cluster string) func(*corev1.Node) {
	return func(n *corev1.Node) {
		n.Annotations = map[string]string{
			capiMachineAnnotation:     cluster + "-worker-abcde",
			capiClusterNameAnnotation: cluster,
		}
	}
}

func healthCheck(cluster string, expected, healthy int64) *unstructured.Unstructured {
	check := &unstructured.Unstructured{Object: map[string]any{
		"spec":   map[string]any{"clusterName": cluster},
		"status": map[string]any{"expectedMachines": expected, "currentHealthy": healthy},
	}}

	check.SetAPIVersion("cluster.x-k8s.io/v1beta2")
	check.SetKind("MachineHealthCheck")
	check.SetNamespace(agentNamespace)
	check.SetName(cluster + "-machine-health-check")

	return check
}

func withoutStatus(obj *unstructured.Unstructured) *unstructured.Unstructured {
	delete(obj.Object, "status")

	return obj
}

func renamed(obj *unstructured.Unstructured, name string) *unstructured.Unstructured {
	obj.SetName(name)

	return obj
}

func instance(name, apiVersion, machine string) *unstructured.Unstructured {
	spec := map[string]any{"nodeRef": map[string]any{"name": nodeName}}
	if machine != "" {
		spec["machineRef"] = map[string]any{
			"kind": "Machine", "apiVersion": apiVersion, "name": machine, "namespace": agentNamespace,
		}
	}

	instance := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	instance.SetAPIVersion("deckhouse.io/v1alpha2")
	instance.SetKind("Instance")
	instance.SetName(name)

	return instance
}

func newMachine() *unstructured.Unstructured {
	machine := &unstructured.Unstructured{}
	machine.SetGroupVersionKind(mcmMachine)

	return machine
}

func machine(name string) *unstructured.Unstructured {
	machine := newMachine()
	machine.SetNamespace(agentNamespace)
	machine.SetName(name)

	return machine
}

func (h *harness) exists(obj client.Object, namespace, name string) bool {
	h.t.Helper()

	err := h.api.Get(h.t.Context(), types.NamespacedName{Namespace: namespace, Name: name}, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		h.t.Fatalf("get %s/%s: %v", namespace, name, err)
	}

	return err == nil
}

func (h *harness) resourceVersion(obj *unstructured.Unstructured) string {
	h.t.Helper()

	if !h.exists(obj, obj.GetNamespace(), obj.GetName()) {
		h.t.Fatalf("%s/%s does not exist", obj.GetNamespace(), obj.GetName())
	}

	return obj.GetResourceVersion()
}

func (h *harness) assertIncidentGone() {
	h.t.Helper()

	if h.exists(&v1alpha1.FencingFailedNodeState{}, "", nodeName) {
		h.t.Errorf("fencingfailednodestate %s is not deleted", nodeName)
	}
}
