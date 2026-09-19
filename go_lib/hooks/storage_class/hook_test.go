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
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkpatchablevalues "github.com/deckhouse/module-sdk/pkg/patchable-values"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type testStorageClass struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

func (sc testStorageClass) name() string { return sc.Name }

func testHookInput(t *testing.T, values map[string]any) *go_hook.HookInput {
	t.Helper()

	input, _ := testHookInputWithPatches(t, values)

	return input
}

// Values.Set and Values.Remove only record a patch — Get keeps returning what the hook was given
// — so a test that checks what the hook published has to read the patches.
func testHookInputWithPatches(t *testing.T, values map[string]any) (*go_hook.HookInput, *sdkpatchablevalues.PatchableValues) {
	t.Helper()

	if values == nil {
		values = map[string]any{}
	}

	patchableValues, err := sdkpatchablevalues.NewPatchableValues(values)
	if err != nil {
		t.Fatalf("NewPatchableValues() = %v", err)
	}

	return &go_hook.HookInput{Values: patchableValues, Logger: log.NewNop()}, patchableValues
}

// assertPatch checks the single patch a test expects. op is "add" or "remove"; for "add" the
// value is compared as the JSON the hook produced.
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

func assertNames(t *testing.T, classes []testStorageClass, want []string) {
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

func testConfig() Config[testStorageClass] {
	return Config[testStorageClass]{
		ModuleName:                    "cloud-provider-test",
		ModuleValuesKey:               "cloudProviderTest",
		DefaultStorageClassValuesPath: "cloudProviderTest.internal.defaultStorageClass",
		NameOfFunc:                    testStorageClass.name,
	}
}

func TestSetDefaultStorageClass(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.IsDefaultFunc = func(sc testStorageClass) bool { return sc.Type == "default" }

	input, values := testHookInputWithPatches(t, map[string]any{
		"cloudProviderTest": map[string]any{"internal": map[string]any{}},
	})

	setDefaultStorageClass(cfg, input, []testStorageClass{
		{Name: "fast"},
		{Name: "slow", Type: "default"},
	})

	assertPatch(t, values, "add", "/cloudProviderTest/internal/defaultStorageClass", `"slow"`)
}

// The default is removed rather than left behind when no class claims it any more — a stale value
// would keep pointing at a class that may have just been excluded.
func TestSetDefaultStorageClassRemovesStaleValue(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.IsDefaultFunc = func(sc testStorageClass) bool { return sc.Type == "default" }

	input, values := testHookInputWithPatches(t, map[string]any{
		"cloudProviderTest": map[string]any{
			"internal": map[string]any{"defaultStorageClass": "gone"},
		},
	})

	setDefaultStorageClass(cfg, input, []testStorageClass{{Name: "fast"}})

	assertPatch(t, values, "remove", "/cloudProviderTest/internal/defaultStorageClass", "")
}

// A module that does not track a default must not have the value invented for it.
func TestSetDefaultStorageClassSkippedWithoutIsDefault(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	input, values := testHookInputWithPatches(t, map[string]any{
		"cloudProviderTest": map[string]any{
			"internal": map[string]any{"defaultStorageClass": "untouched"},
		},
	})

	setDefaultStorageClass(cfg, input, []testStorageClass{{Name: "fast"}})

	if patches := values.GetPatches(); len(patches) != 0 {
		t.Errorf("patches = %+v, want none", patches)
	}
}
