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

package machinesetrevision

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func newReconciler(t *testing.T, objs ...runtime.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(machineSetGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		machineSetGVK.GroupVersion().WithKind("MachineSetList"),
		&unstructured.UnstructuredList{},
	)
	cl := fakeclient.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &Reconciler{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}
}

func machineSet(name string, annotations map[string]string) *unstructured.Unstructured {
	u := newMachineSet()
	u.SetNamespace(nodecommon.MachineNamespace)
	u.SetName(name)
	if annotations != nil {
		u.SetAnnotations(annotations)
	}
	return u
}

func doReconcile(t *testing.T, r *Reconciler, name string) {
	t.Helper()
	doReconcileNS(t, r, nodecommon.MachineNamespace, name)
}

func doReconcileNS(t *testing.T, r *Reconciler, ns, name string) {
	t.Helper()
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: ns, Name: name}})
	if err != nil {
		t.Fatalf("reconcile %s/%s: %v", ns, name, err)
	}
}

func revisionHistory(t *testing.T, r *Reconciler, name string) string {
	t.Helper()
	u := newMachineSet()
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, u); err != nil {
		t.Fatalf("get machineset %s: %v", name, err)
	}
	return u.GetAnnotations()[revisionHistoryKey]
}

// A long comma-separated list (>16 chars) collapses to the first revision.
func TestReconcile_LongHistory_Trimmed(t *testing.T) {
	r := newReconciler(t, machineSet("long", map[string]string{
		revisionHistoryKey: "1,2,3,4,5,6,7,8,9",
		"other-annotation": "value",
	}))
	doReconcile(t, r, "long")

	if got := revisionHistory(t, r, "long"); got != "1" {
		t.Fatalf("expected trimmed to '1', got %q", got)
	}
	// Other annotations are preserved by the merge patch.
	u := newMachineSet()
	_ = r.Client.Get(context.Background(), types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: "long"}, u)
	if u.GetAnnotations()["other-annotation"] != "value" {
		t.Fatalf("expected other-annotation preserved, got %v", u.GetAnnotations())
	}
}

// A short value (<=16 chars) is left untouched.
func TestReconcile_ShortHistory_Unchanged(t *testing.T) {
	r := newReconciler(t, machineSet("short", map[string]string{revisionHistoryKey: "1,2,3"}))
	doReconcile(t, r, "short")

	if got := revisionHistory(t, r, "short"); got != "1,2,3" {
		t.Fatalf("expected unchanged '1,2,3', got %q", got)
	}
}

// A value exactly at the boundary (15 chars, <=16) is left untouched.
func TestReconcile_BoundaryHistory_Unchanged(t *testing.T) {
	r := newReconciler(t, machineSet("boundary", map[string]string{revisionHistoryKey: "1,2,3,4,5,6,7,8"}))
	doReconcile(t, r, "boundary")

	if got := revisionHistory(t, r, "boundary"); got != "1,2,3,4,5,6,7,8" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

// A long value without a comma has nothing to trim, so it is left untouched.
func TestReconcile_LongWithoutComma_Unchanged(t *testing.T) {
	r := newReconciler(t, machineSet("nocomma", map[string]string{revisionHistoryKey: "12345678901234567"}))
	doReconcile(t, r, "nocomma")

	if got := revisionHistory(t, r, "nocomma"); got != "12345678901234567" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

// An empty annotation is left untouched.
func TestReconcile_EmptyHistory_Unchanged(t *testing.T) {
	r := newReconciler(t, machineSet("empty", map[string]string{revisionHistoryKey: ""}))
	doReconcile(t, r, "empty")

	if got := revisionHistory(t, r, "empty"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// A MachineSet without the annotation is left untouched.
func TestReconcile_AbsentHistory_Unchanged(t *testing.T) {
	r := newReconciler(t, machineSet("absent", nil))
	doReconcile(t, r, "absent")

	if _, ok := revisionAnnotations(t, r, "absent")[revisionHistoryKey]; ok {
		t.Fatal("expected revision-history annotation to remain absent")
	}
}

// MachineSets outside d8-cloud-instance-manager are ignored (namespace parity with the hook).
func TestReconcile_OtherNamespace_Ignored(t *testing.T) {
	u := newMachineSet()
	u.SetNamespace("default")
	u.SetName("elsewhere")
	u.SetAnnotations(map[string]string{revisionHistoryKey: "1,2,3,4,5,6,7,8,9"})
	r := newReconciler(t, u)
	doReconcileNS(t, r, "default", "elsewhere")

	got := newMachineSet()
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "elsewhere"}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetAnnotations()[revisionHistoryKey] != "1,2,3,4,5,6,7,8,9" {
		t.Fatalf("expected untouched in other namespace, got %q", got.GetAnnotations()[revisionHistoryKey])
	}
}

func TestReconcile_NotFound_NoError(t *testing.T) {
	r := newReconciler(t)
	doReconcile(t, r, "nonexistent")
}

func TestTrimRevisionHistory(t *testing.T) {
	cases := map[string]string{
		"1,2,3,4,5":         "1",
		"12345678901234567": "12345678901234567",
		"":                  "",
		"7":                 "7",
	}
	for in, want := range cases {
		if got := trimRevisionHistory(in); got != want {
			t.Fatalf("trim(%q) = %q, want %q", in, got, want)
		}
	}
}

func revisionAnnotations(t *testing.T, r *Reconciler, name string) map[string]string {
	t.Helper()
	u := newMachineSet()
	if err := r.Client.Get(context.Background(), types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}, u); err != nil {
		t.Fatalf("get machineset %s: %v", name, err)
	}
	return u.GetAnnotations()
}
