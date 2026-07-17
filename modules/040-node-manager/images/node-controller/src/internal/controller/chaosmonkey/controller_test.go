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

package chaosmonkey

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("corev1 scheme: %v", err)
	}
	if err := deckhousev1.AddToScheme(scheme); err != nil {
		t.Fatalf("deckhousev1 scheme: %v", err)
	}
	scheme.AddKnownTypeWithName(machineGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(machineGVK.GroupVersion().WithKind("MachineList"), &unstructured.UnstructuredList{})

	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func nodeGroup(name string, cloud bool, mode deckhousev1.ChaosMode, period string, total, ready int32) *deckhousev1.NodeGroup {
	ng := &deckhousev1.NodeGroup{}
	ng.Name = name
	if cloud {
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudEphemeral
		ng.Status.Desired = total
	} else {
		ng.Spec.NodeType = deckhousev1.NodeTypeCloudPermanent
		ng.Status.Nodes = total
	}
	ng.Status.Ready = ready
	if mode != "" || period != "" {
		ng.Spec.Chaos = &deckhousev1.ChaosSpec{Mode: mode, Period: period}
	}
	return ng
}

func node(name, group string) *corev1.Node {
	n := &corev1.Node{}
	n.Name = name
	n.Labels = map[string]string{nodeGroupLabel: group}
	return n
}

func machine(name, nodeName string, victim bool) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(machineGVK)
	u.SetNamespace(nodecommon.MachineNamespace)
	u.SetName(name)
	labels := map[string]string{machineNodeLabel: nodeName}
	if victim {
		labels[victimKey] = ""
	}
	u.SetLabels(labels)
	return u
}

func runReconcile(t *testing.T, r *Reconciler) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func machineExists(t *testing.T, r *Reconciler, name string) bool {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(machineGVK)
	err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, u)
	return err == nil
}

// largeCluster: a chaos-enabled ready NG "largeng" with 3 nodes/machines plus a
// non-chaos single-node "too-small" NG. cloud toggles CloudEphemeral vs CloudPermanent.
func largeCluster(cloud bool, mode deckhousev1.ChaosMode, period string, ready int32) []runtime.Object {
	return []runtime.Object{
		nodeGroup("largeng", cloud, mode, period, 3, ready),
		nodeGroup("too-small", cloud, "", "", 1, 1),
		node("node1", "largeng"), node("node2", "largeng"), node("node3", "largeng"),
		node("smallnode1", "too-small"),
		machine("node1", "node1", false),
		machine("node2", "node2", false),
		machine("node3", "node3", false),
		machine("smallnode1", "smallnode1", false),
	}
}

func countLargeMachines(t *testing.T, r *Reconciler) int {
	t.Helper()
	n := 0
	for _, name := range []string{"node1", "node2", "node3"} {
		if machineExists(t, r, name) {
			n++
		}
	}
	return n
}

// An empty cluster reconciles without error.
func TestReconcile_Empty_NoError(t *testing.T) {
	r := newReconciler(t)
	runReconcile(t, r)
}

// A ready chaos NG with period 1m always passes the probability gate, so exactly one
// of its machines is deleted; the non-chaos NG's machine is untouched.
func TestReconcile_Lucky_OneDeleted(t *testing.T) {
	for _, cloud := range []bool{true, false} {
		t.Run(map[bool]string{true: "cloud", false: "hybrid"}[cloud], func(t *testing.T) {
			t.Setenv("D8_TEST_RANDOM_SEED", "11")
			r := newReconciler(t, largeCluster(cloud, deckhousev1.ChaosModeDrainAndDelete, "1m", 3)...)
			runReconcile(t, r)

			if got := countLargeMachines(t, r); got != 2 {
				t.Fatalf("expected exactly one largeng machine deleted (2 remain), got %d remaining", got)
			}
			if !machineExists(t, r, "smallnode1") {
				t.Fatal("non-chaos NG machine must survive")
			}
		})
	}
}

// seed 0 with period 5m fails the probability gate — nothing is deleted (parity with
// the hook's "isn't lucky" test).
func TestReconcile_Unlucky_NoneDeleted(t *testing.T) {
	t.Setenv("D8_TEST_RANDOM_SEED", "0")
	r := newReconciler(t, largeCluster(true, deckhousev1.ChaosModeDrainAndDelete, "5m", 3)...)
	runReconcile(t, r)

	if got := countLargeMachines(t, r); got != 3 {
		t.Fatalf("expected all machines to survive, got %d remaining", got)
	}
}

// A NG whose ready count is below desired is not ready for chaos — nothing is deleted.
func TestReconcile_NotReady_NoneDeleted(t *testing.T) {
	t.Setenv("D8_TEST_RANDOM_SEED", "11")
	r := newReconciler(t, largeCluster(true, deckhousev1.ChaosModeDrainAndDelete, "1m", 2)...)
	runReconcile(t, r)

	if got := countLargeMachines(t, r); got != 3 {
		t.Fatalf("expected all machines to survive on a non-ready NG, got %d remaining", got)
	}
}

// If any machine is already flagged as a victim, the global gate stops all deletions.
func TestReconcile_ExistingVictim_NoneDeleted(t *testing.T) {
	t.Setenv("D8_TEST_RANDOM_SEED", "11")
	objs := append(largeCluster(true, deckhousev1.ChaosModeDrainAndDelete, "1m", 3),
		machine("victimnode", "victimnode", true))
	r := newReconciler(t, objs...)
	runReconcile(t, r)

	if got := countLargeMachines(t, r); got != 3 {
		t.Fatalf("expected all machines to survive while a victim exists, got %d remaining", got)
	}
}

// Chaos mode other than DrainAndDelete (e.g. Disabled) deletes nothing.
func TestReconcile_ModeDisabled_NoneDeleted(t *testing.T) {
	t.Setenv("D8_TEST_RANDOM_SEED", "11")
	r := newReconciler(t, largeCluster(true, deckhousev1.ChaosModeDisabled, "1m", 3)...)
	runReconcile(t, r)

	if got := countLargeMachines(t, r); got != 3 {
		t.Fatalf("expected all machines to survive with mode Disabled, got %d remaining", got)
	}
}

// A sub-minute period is skipped instead of panicking on a zero modulus.
func TestReconcile_SubMinutePeriod_Skipped(t *testing.T) {
	t.Setenv("D8_TEST_RANDOM_SEED", "11")
	r := newReconciler(t, largeCluster(true, deckhousev1.ChaosModeDrainAndDelete, "30s", 3)...)
	runReconcile(t, r)

	if got := countLargeMachines(t, r); got != 3 {
		t.Fatalf("expected all machines to survive with a sub-minute period, got %d remaining", got)
	}
}

func TestIsReadyForChaos(t *testing.T) {
	cases := []struct {
		name  string
		ng    *deckhousev1.NodeGroup
		ready bool
	}{
		{"cloud ready", nodeGroup("a", true, deckhousev1.ChaosModeDrainAndDelete, "1m", 3, 3), true},
		{"cloud not full", nodeGroup("a", true, deckhousev1.ChaosModeDrainAndDelete, "1m", 3, 2), false},
		{"cloud single", nodeGroup("a", true, deckhousev1.ChaosModeDrainAndDelete, "1m", 1, 1), false},
		{"hybrid ready", nodeGroup("a", false, deckhousev1.ChaosModeDrainAndDelete, "1m", 3, 3), true},
		{"hybrid single", nodeGroup("a", false, deckhousev1.ChaosModeDrainAndDelete, "1m", 1, 1), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isReadyForChaos(c.ng); got != c.ready {
				t.Fatalf("isReadyForChaos = %v, want %v", got, c.ready)
			}
		})
	}
}
