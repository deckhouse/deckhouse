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

package storage_class

import (
	"context"
	"errors"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type testStorageClass struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

func (sc testStorageClass) GetName() string { return sc.Name }

// fakePatchCollector records the StorageClasses a step deletes; nothing else is expected of it.
type fakePatchCollector struct {
	go_hook.PatchCollector
	deleted []string
}

func (c *fakePatchCollector) Delete(_, _, _, name string) {
	c.deleted = append(c.deleted, name)
}

// testState builds the state of one run. Values.Set and Values.Remove only record a patch — Get keeps
// returning what the hook was given — so a test that checks what a step published reads the patches.
func testState(t *testing.T, values map[string]any, actual ...storagev1.StorageClass) (*State[testStorageClass], *sdkpatchablevalues.PatchableValues, *fakePatchCollector) {
	t.Helper()

	if values == nil {
		values = map[string]any{}
	}

	patchableValues, err := sdkpatchablevalues.NewPatchableValues(values)
	if err != nil {
		t.Fatalf("NewPatchableValues() = %v", err)
	}

	patches := &fakePatchCollector{}
	input := &go_hook.HookInput{Values: patchableValues, Logger: log.NewNop(), PatchCollector: patches}

	return &State[testStorageClass]{
		Input:      input,
		ModuleName: "cloud-provider-test",
		Actual:     actual,
	}, patchableValues, patches
}

func testExistingStorageClass(name, classType string) storagev1.StorageClass {
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Parameters: map[string]string{"type": classType},
	}
}

// testConvertStorageClass renders an object in the cluster the way a module renders its desired classes.
func testConvertStorageClass(sc storagev1.StorageClass) testStorageClass {
	return testStorageClass{Name: sc.Name, Type: sc.Parameters["type"]}
}

func mustRun(t *testing.T, state *State[testStorageClass], steps ...Step[testStorageClass]) {
	t.Helper()

	if err := Run(context.Background(), state, steps...); err != nil {
		t.Fatalf("Run() = %v", err)
	}
}

func assertDesired(t *testing.T, got []testStorageClass, want ...testStorageClass) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("desired = %+v, want %+v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("desired = %+v, want %+v", got, want)
		}
	}
}

func assertNames(t *testing.T, classes []testStorageClass, want ...string) {
	t.Helper()

	got := make([]string, 0, len(classes))
	for _, class := range classes {
		got = append(got, class.Name)
	}

	if len(got) != len(want) {
		t.Fatalf("names = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("names = %v, want %v", got, want)
		}
	}
}

// assertPatch checks the single patch a test expects. op is "add" or "remove"; for "add" the
// value is compared as the JSON the step produced.
func assertPatch(t *testing.T, values *sdkpatchablevalues.PatchableValues, op, path, value string) {
	t.Helper()

	patches := values.GetPatches()
	if len(patches) != 1 {
		t.Fatalf("patches = %d, want 1: %+v", len(patches), patches)
	}

	patch := patches[0]
	if patch.Op != op || patch.Path != path {
		t.Fatalf("patch = %s %s, want %s %s", patch.Op, patch.Path, op, path)
	}

	if op == "add" && string(patch.Value) != value {
		t.Errorf("patch value = %s, want %s", patch.Value, value)
	}
}

func assertNoPatches(t *testing.T, values *sdkpatchablevalues.PatchableValues) {
	t.Helper()

	if patches := values.GetPatches(); len(patches) != 0 {
		t.Errorf("patches = %+v, want none", patches)
	}
}

func TestRunExecutesStepsInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	step := func(name string) Step[testStorageClass] {
		return func(context.Context, *State[testStorageClass]) error {
			order = append(order, name)
			return nil
		}
	}

	state, _, _ := testState(t, nil)
	mustRun(t, state, step("first"), step("second"), step("third"))

	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Fatalf("order = %v, want [first second third]", order)
	}
}

// ErrStop ends the run without failing it: the steps after it do not run.
func TestRunTreatsErrStopAsSuccess(t *testing.T) {
	t.Parallel()

	ran := false
	state, _, _ := testState(t, nil)
	mustRun(t, state,
		func(context.Context, *State[testStorageClass]) error { return ErrStop },
		func(context.Context, *State[testStorageClass]) error { ran = true; return nil },
	)

	if ran {
		t.Error("a step after ErrStop ran")
	}
}

// A step may wrap ErrStop with context; the run still ends successfully.
func TestRunTreatsWrappedErrStopAsSuccess(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state, func(context.Context, *State[testStorageClass]) error {
		return errors.Join(errors.New("nothing to do"), ErrStop)
	})
}

func TestRunReturnsStepErrors(t *testing.T) {
	t.Parallel()

	ran := false
	boom := errors.New("boom")
	state, _, _ := testState(t, nil)
	err := Run(context.Background(), state,
		func(context.Context, *State[testStorageClass]) error { return boom },
		func(context.Context, *State[testStorageClass]) error { ran = true; return nil },
	)

	if !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want %v", err, boom)
	}
	if ran {
		t.Error("a step after a failed one ran")
	}
}

func TestRunWithoutSteps(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, nil)
	mustRun(t, state)

	assertNoPatches(t, values)
}

func TestFilterStorageClass(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "storage.k8s.io/v1",
		"kind":       "StorageClass",
		"metadata":   map[string]any{"name": "fast"},
		"parameters": map[string]any{"type": "ssd"},
	}}

	result, err := filterStorageClass(obj)
	if err != nil {
		t.Fatalf("filterStorageClass() = %v", err)
	}

	storageClass, ok := result.(*storagev1.StorageClass)
	if !ok {
		t.Fatalf("filterStorageClass() = %T, want *storagev1.StorageClass", result)
	}
	if storageClass.Name != "fast" || storageClass.Parameters["type"] != "ssd" {
		t.Fatalf("filterStorageClass() = %+v, want fast/ssd", storageClass)
	}
}

func TestFilterStorageClassRejectsAMalformedObject(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "storage.k8s.io/v1",
		"kind":       "StorageClass",
		"metadata":   map[string]any{"name": "broken"},
		"parameters": "not a map",
	}}

	if _, err := filterStorageClass(obj); err == nil {
		t.Fatal("filterStorageClass() = nil, want the malformed object to be reported")
	}
}
