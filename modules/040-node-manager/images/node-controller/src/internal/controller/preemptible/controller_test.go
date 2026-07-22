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

package preemptible

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	scheme.AddKnownTypeWithName(yandexMachineClassGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(yandexMachineClassGVK.GroupVersion().WithKind("YandexMachineClassList"), &unstructured.UnstructuredList{})

	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func preemptibleClass(name string, preemptible bool) *unstructured.Unstructured {
	c := &unstructured.Unstructured{}
	c.SetGroupVersionKind(yandexMachineClassGVK)
	c.SetNamespace(nodecommon.MachineNamespace)
	c.SetName(name)
	_ = unstructured.SetNestedField(c.Object, preemptible, "spec", "schedulingPolicy", "preemptible")
	return c
}

func machineWithKind(name, kind, className string) *unstructured.Unstructured {
	m := &unstructured.Unstructured{}
	m.SetGroupVersionKind(machineGVK)
	m.SetNamespace(nodecommon.MachineNamespace)
	m.SetName(name)
	_ = unstructured.SetNestedField(m.Object, kind, "spec", "class", "kind")
	_ = unstructured.SetNestedField(m.Object, className, "spec", "class", "name")
	return m
}

func machine(name, className string) *unstructured.Unstructured {
	return machineWithKind(name, yandexMachineClassKind, className)
}

func terminatingMachine(name, className string) *unstructured.Unstructured {
	m := machine(name, className)
	now := metav1.Now()
	m.SetDeletionTimestamp(&now)
	m.SetFinalizers([]string{"test.deckhouse.io/hold"})
	return m
}

func node(name, group string, age time.Duration) *corev1.Node {
	n := &corev1.Node{}
	n.Name = name
	n.Labels = map[string]string{nodeGroupLabel: group}
	n.CreationTimestamp = metav1.NewTime(time.Now().Add(-age))
	return n
}

func ycNodeGroup(name string, nodes, ready int32) *deckhousev1.NodeGroup {
	ng := &deckhousev1.NodeGroup{}
	ng.Name = name
	ng.Spec.CloudInstances = &deckhousev1.CloudInstancesSpec{
		ClassReference: deckhousev1.ClassReference{Kind: yandexInstanceClassKind, Name: "ic"},
	}
	ng.Status.Nodes = nodes
	ng.Status.Ready = ready
	return ng
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

func countMachines(t *testing.T, r *Reconciler, names []string) int {
	t.Helper()
	n := 0
	for _, name := range names {
		if machineExists(t, r, name) {
			n++
		}
	}
	return n
}

const oldAge = 21 * time.Hour  // past the 20h (24h-4h) threshold
const youngAge = 10 * time.Hour

// preemptibleCluster: n old preemptible Machines/Nodes in one ready Yandex NG "yng".
func preemptibleCluster(n int, ready int32) []runtime.Object {
	objs := []runtime.Object{
		preemptibleClass("pre-class", true),
		ycNodeGroup("yng", int32(n), ready),
	}
	for i := range n {
		name := fmt.Sprintf("m%02d", i)
		// stagger ages so ordering is deterministic: m00 is oldest.
		age := oldAge + time.Duration(n-i)*time.Hour
		objs = append(objs, machine(name, "pre-class"), node(name, "yng", age))
	}
	return objs
}

func clusterMachineNames(n int) []string {
	names := make([]string, n)
	for i := range n {
		names[i] = fmt.Sprintf("m%02d", i)
	}
	return names
}

// An empty cluster reconciles without error.
func TestReconcile_Empty_NoError(t *testing.T) {
	r := newReconciler(t)
	runReconcile(t, r)
}

// No preemptible YandexMachineClass → nothing is deleted even with old nodes.
func TestReconcile_NoPreemptibleClass_NoneDeleted(t *testing.T) {
	objs := []runtime.Object{
		preemptibleClass("pre-class", false),
		ycNodeGroup("yng", 1, 1),
		machine("m00", "pre-class"), node("m00", "yng", oldAge),
	}
	r := newReconciler(t, objs...)
	runReconcile(t, r)

	if !machineExists(t, r, "m00") {
		t.Fatal("no preemptible class present → machine must survive")
	}
}

// Old-enough preemptible Machines in a ready NG are rotated 10% at a time, oldest first.
func TestReconcile_OldPreemptible_BatchDeleted(t *testing.T) {
	r := newReconciler(t, preemptibleCluster(20, 20)...)
	runReconcile(t, r)

	names := clusterMachineNames(20)
	if got := countMachines(t, r, names); got != 18 {
		t.Fatalf("expected 10%% (2) of 20 machines deleted (18 remain), got %d remaining", got)
	}
	// The two oldest (m00, m01) must be the deleted ones.
	if machineExists(t, r, "m00") || machineExists(t, r, "m01") {
		t.Fatal("the two oldest machines must be the ones deleted")
	}
}

// Fewer than 10 candidates still deletes exactly one (batch floor of 1).
func TestReconcile_SmallBatch_DeletesOne(t *testing.T) {
	r := newReconciler(t, preemptibleCluster(3, 3)...)
	runReconcile(t, r)

	if got := countMachines(t, r, clusterMachineNames(3)); got != 2 {
		t.Fatalf("expected exactly one machine deleted (2 remain), got %d remaining", got)
	}
	if machineExists(t, r, "m00") {
		t.Fatal("the oldest machine must be deleted")
	}
}

// Young nodes (below the 20h threshold) are never rotated.
func TestReconcile_YoungNodes_NoneDeleted(t *testing.T) {
	objs := []runtime.Object{
		preemptibleClass("pre-class", true),
		ycNodeGroup("yng", 3, 3),
	}
	for _, name := range []string{"m00", "m01", "m02"} {
		objs = append(objs, machine(name, "pre-class"), node(name, "yng", youngAge))
	}
	r := newReconciler(t, objs...)
	runReconcile(t, r)

	if got := countMachines(t, r, clusterMachineNames(3)); got != 3 {
		t.Fatalf("expected all young machines to survive, got %d remaining", got)
	}
}

// A NodeGroup below the 0.9 ready ratio is protected — nothing is rotated in it.
func TestReconcile_LowRatio_NoneDeleted(t *testing.T) {
	// 10 nodes, 8 ready → 0.8 < 0.9.
	r := newReconciler(t, preemptibleCluster(10, 8)...)
	runReconcile(t, r)

	if got := countMachines(t, r, clusterMachineNames(10)); got != 10 {
		t.Fatalf("expected all machines to survive under the ready ratio, got %d remaining", got)
	}
}

// A Machine already terminating is not counted as a candidate.
func TestReconcile_Terminating_Skipped(t *testing.T) {
	objs := []runtime.Object{
		preemptibleClass("pre-class", true),
		ycNodeGroup("yng", 1, 1),
		terminatingMachine("m00", "pre-class"), node("m00", "yng", oldAge),
	}
	r := newReconciler(t, objs...)
	runReconcile(t, r)

	// Still present (only had a deletionTimestamp+finalizer, our controller doesn't re-delete).
	if !machineExists(t, r, "m00") {
		t.Fatal("terminating machine must not be treated as a fresh candidate")
	}
}

// A Machine backed by a non-Yandex class is ignored.
func TestReconcile_NonYandexClass_Skipped(t *testing.T) {
	objs := []runtime.Object{
		preemptibleClass("pre-class", true),
		ycNodeGroup("yng", 1, 1),
		machineWithKind("m00", "AWSMachineClass", "pre-class"), node("m00", "yng", oldAge),
	}
	r := newReconciler(t, objs...)
	runReconcile(t, r)

	if !machineExists(t, r, "m00") {
		t.Fatal("non-Yandex machine class must be ignored")
	}
}

func TestMachinesToDelete(t *testing.T) {
	now := time.Now()
	mk := func(n int) []deletionCandidate {
		out := make([]deletionCandidate, n)
		for i := range n {
			// index i created i hours ago → larger i is older.
			out[i] = deletionCandidate{
				name:                  fmt.Sprintf("m%02d", i),
				nodeCreationTimestamp: now.Add(-time.Duration(i) * time.Hour),
			}
		}
		return out
	}

	cases := []struct {
		name  string
		count int
		want  int
	}{
		{"one", 1, 1},
		{"nine floor to one", 9, 1},
		{"ten is ten percent", 10, 1},
		{"twenty is ten percent", 20, 2},
		{"twenty five", 25, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := machinesToDelete(mk(c.count))
			if len(got) != c.want {
				t.Fatalf("batch size = %d, want %d", len(got), c.want)
			}
			// oldest first: highest index appears first.
			if len(got) > 0 {
				oldest := fmt.Sprintf("m%02d", c.count-1)
				if got[0] != oldest {
					t.Fatalf("expected oldest %q first, got %q", oldest, got[0])
				}
			}
		})
	}
}
