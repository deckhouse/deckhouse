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

	storagev1 "k8s.io/api/storage/v1"
)

func TestAppendKeepsWhatIsAlreadyDesired(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a"})),
		Append(Static(testStorageClass{Name: "b"})),
	)

	assertNames(t, state.Desired, "a", "b")
}

func TestAppendReturnsSourceErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	failing := func(context.Context, *State[testStorageClass]) ([]testStorageClass, error) { return nil, boom }

	state, _, _ := testState(t, nil)
	if err := Run(context.Background(), state, Append[testStorageClass](failing)); !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want %v", err, boom)
	}
}

// The provisioned classes of Yandex Cloud: an entry overrides the default class with exactly its
// name, in place, and the rest are added in their order — the default names are prefixes of each
// other, so a prefix must not override anything.
func TestOverrideByNameReplacesExactNamesOnly(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(
			testStorageClass{Name: "network-hdd", Type: "network-hdd"},
			testStorageClass{Name: "network-ssd", Type: "network-ssd"},
			testStorageClass{Name: "network-ssd-nonreplicated", Type: "network-ssd-nonreplicated"},
		)),
		OverrideByName(Static(
			testStorageClass{Name: "network-ssd-io-m3-128k", Type: "network-ssd-io-m3"},
			testStorageClass{Name: "network-ssd", Type: "network-ssd-64k"},
			testStorageClass{Name: "network-ssd-64k", Type: "network-ssd"},
		)),
	)

	assertDesired(t, state.Desired,
		testStorageClass{Name: "network-hdd", Type: "network-hdd"},
		testStorageClass{Name: "network-ssd", Type: "network-ssd-64k"},
		testStorageClass{Name: "network-ssd-nonreplicated", Type: "network-ssd-nonreplicated"},
		testStorageClass{Name: "network-ssd-io-m3-128k", Type: "network-ssd-io-m3"},
		testStorageClass{Name: "network-ssd-64k", Type: "network-ssd"},
	)
}

func TestOverrideByNameIntoAnEmptySet(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state, OverrideByName(Static(testStorageClass{Name: "a"}, testStorageClass{Name: "b"})))

	assertNames(t, state.Desired, "a", "b")
}

func TestOverrideByNameWithAnEmptySourceKeepsTheSet(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a", Type: "x"})),
		OverrideByName(Static[testStorageClass]()),
	)

	assertDesired(t, state.Desired, testStorageClass{Name: "a", Type: "x"})
}

// Duplicate names in the source are rejected by the module's validation; if they get through, the
// last one wins, both for an override and for an addition, and the name appears once.
func TestOverrideByNameDuplicatesInTheSourceCollapseToTheLast(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a", Type: "default"})),
		OverrideByName(Static(
			testStorageClass{Name: "a", Type: "first"},
			testStorageClass{Name: "a", Type: "last"},
			testStorageClass{Name: "b", Type: "first"},
			testStorageClass{Name: "b", Type: "last"},
		)),
	)

	assertDesired(t, state.Desired,
		testStorageClass{Name: "a", Type: "last"},
		testStorageClass{Name: "b", Type: "last"},
	)
}

func TestOverrideByNameReturnsSourceErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	failing := func(context.Context, *State[testStorageClass]) ([]testStorageClass, error) { return nil, boom }

	state, _, _ := testState(t, nil)
	mustRun(t, state, Append(Static(testStorageClass{Name: "a"})))
	if err := Run(context.Background(), state, OverrideByName[testStorageClass](failing)); !errors.Is(err, boom) {
		t.Fatalf("Run() = %v, want %v", err, boom)
	}

	assertNames(t, state.Desired, "a")
}

func TestExclude(t *testing.T) {
	t.Parallel()

	desired := []testStorageClass{
		{Name: "network-hdd"},
		{Name: "network-ssd"},
		{Name: "network-ssd-nonreplicated"},
		{Name: "network-ssd-io-m3"},
	}

	tests := []struct {
		name     string
		patterns []any
		want     []string
	}{
		{
			name:     "a pattern matches the whole name",
			patterns: []any{".*-hdd"},
			want:     []string{"network-ssd", "network-ssd-nonreplicated", "network-ssd-io-m3"},
		},
		{
			// Anchoring keeps network-ssd from excluding the classes it is a prefix of.
			name:     "an exact name is not a prefix",
			patterns: []any{"network-ssd"},
			want:     []string{"network-hdd", "network-ssd-nonreplicated", "network-ssd-io-m3"},
		},
		{
			name:     "several patterns",
			patterns: []any{"network-hdd", "network-ssd-.*"},
			want:     []string{"network-ssd"},
		},
		{
			name:     "an alternation is anchored as a whole",
			patterns: []any{"network-hdd|network-ssd"},
			want:     []string{"network-ssd-nonreplicated", "network-ssd-io-m3"},
		},
		{
			name: "no patterns",
			want: []string{"network-hdd", "network-ssd", "network-ssd-nonreplicated", "network-ssd-io-m3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			values := map[string]any{"cloudProviderTest": map[string]any{}}
			if tt.patterns != nil {
				values["cloudProviderTest"] = map[string]any{"exclude": tt.patterns}
			}

			state, _, _ := testState(t, values)
			state.Desired = append([]testStorageClass(nil), desired...)
			mustRun(t, state, Exclude[testStorageClass]("cloudProviderTest.exclude"))

			assertNames(t, state.Desired, tt.want...)
		})
	}
}

// Exclude runs on whatever the earlier steps produced, so it filters the provisioned classes too.
func TestExcludeAfterOverride(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, map[string]any{
		"cloudProviderTest": map[string]any{"exclude": []any{"network-ssd.*"}},
	})
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "network-hdd"}, testStorageClass{Name: "network-ssd"})),
		OverrideByName(Static(testStorageClass{Name: "network-ssd-64k"})),
		Exclude[testStorageClass]("cloudProviderTest.exclude"),
	)

	assertNames(t, state.Desired, "network-hdd")
}

func TestExcludeRejectsAnInvalidPattern(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, map[string]any{
		"cloudProviderTest": map[string]any{"exclude": []any{"("}},
	})
	state.Desired = []testStorageClass{{Name: "a"}}
	if err := Run(context.Background(), state, Exclude[testStorageClass]("cloudProviderTest.exclude")); err == nil {
		t.Fatal("Run() = nil, want the invalid pattern to be reported")
	}

	assertNames(t, state.Desired, "a")
}

func TestFilter(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a", Type: "ssd"}, testStorageClass{Name: "b", Type: "hdd"}, testStorageClass{Name: "c", Type: "ssd"})),
		Filter(func(sc testStorageClass) bool { return sc.Type == "ssd" }),
	)

	assertNames(t, state.Desired, "a", "c")
}

func TestMap(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a"}, testStorageClass{Name: "b"})),
		Map(func(sc testStorageClass) testStorageClass { sc.Type = "ssd"; return sc }),
	)

	assertDesired(t, state.Desired, testStorageClass{Name: "a", Type: "ssd"}, testStorageClass{Name: "b", Type: "ssd"})
}

// Static hands its own slice to Append; Map must not reach back into the source's data.
func TestMapDoesNotChangeTheStaticSource(t *testing.T) {
	t.Parallel()

	defaults := []testStorageClass{{Name: "a"}}

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(defaults...)),
		Map(func(sc testStorageClass) testStorageClass { sc.Type = "changed"; return sc }),
	)

	if defaults[0].Type != "" {
		t.Fatalf("defaults = %+v, want the source left as it was", defaults)
	}
}

func TestSortByNameIsStable(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		Append(Static(
			testStorageClass{Name: "b"},
			testStorageClass{Name: "a", Type: "first"},
			testStorageClass{Name: "a", Type: "second"},
		)),
		SortByName[testStorageClass](),
	)

	assertDesired(t, state.Desired,
		testStorageClass{Name: "a", Type: "first"},
		testStorageClass{Name: "a", Type: "second"},
		testStorageClass{Name: "b"},
	)
}

func TestSkipIf(t *testing.T) {
	t.Parallel()

	for _, skip := range []bool{true, false} {
		state, values, _ := testState(t, map[string]any{"cloudProviderTest": map[string]any{"internal": map[string]any{}}})
		mustRun(t, state,
			SkipIf(func(*State[testStorageClass]) bool { return skip }),
			Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
		)

		if got := len(values.GetPatches()) == 0; got != skip {
			t.Errorf("skip=%v: patches = %+v", skip, values.GetPatches())
		}
	}
}

// StopIfEmpty keeps the previous values and the cluster while there is nothing to publish.
func TestStopIfEmpty(t *testing.T) {
	t.Parallel()

	state, values, patches := testState(t, nil, testExistingStorageClass("fast", "ssd"))
	mustRun(t, state,
		StopIfEmpty[testStorageClass](),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
		Prune(func(_ storagev1.StorageClass, _ *testStorageClass) bool { return true }),
	)

	assertNoPatches(t, values)
	if len(patches.deleted) != 0 {
		t.Errorf("deleted = %v, want none", patches.deleted)
	}
}

func TestStopIfEmptyContinuesWithDesiredClasses(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, map[string]any{"cloudProviderTest": map[string]any{"internal": map[string]any{}}})
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a"})),
		StopIfEmpty[testStorageClass](),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
	)

	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses", `[{"name":"a"}]`)
}

func TestWhen(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	mustRun(t, state,
		When(func(*State[testStorageClass]) bool { return false }, Append(Static(testStorageClass{Name: "no"}))),
		When(func(*State[testStorageClass]) bool { return true },
			Append(Static(testStorageClass{Name: "yes"})),
			Append(Static(testStorageClass{Name: "also"})),
		),
	)

	assertNames(t, state.Desired, "yes", "also")
}

// ErrStop inside When ends the whole run, not just the nested steps.
func TestWhenPropagatesErrStop(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, map[string]any{"cloudProviderTest": map[string]any{"internal": map[string]any{}}})
	mustRun(t, state,
		When(func(*State[testStorageClass]) bool { return true }, StopIfEmpty[testStorageClass]()),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
	)

	assertNoPatches(t, values)
}

// taggedStorageClass carries a slice, so it is not comparable: every step except PruneModified has to
// accept it.
type taggedStorageClass struct {
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

func (sc taggedStorageClass) GetName() string { return sc.Name }

func TestStepsAcceptNonComparableEntries(t *testing.T) {
	t.Parallel()

	base, values, _ := testState(t, map[string]any{
		"cloudProviderTest": map[string]any{
			"internal":    map[string]any{},
			"provisioned": []any{map[string]any{"name": "a", "tags": []any{"x"}}},
		},
	})
	state := &State[taggedStorageClass]{Input: base.Input, ModuleName: base.ModuleName}

	err := Run(context.Background(), state,
		Append(Static(taggedStorageClass{Name: "b"}, taggedStorageClass{Name: "a"})),
		OverrideByName(FromValues[taggedStorageClass]("cloudProviderTest.provisioned")),
		Filter(func(taggedStorageClass) bool { return true }),
		SortByName[taggedStorageClass](),
		Publish[taggedStorageClass]("cloudProviderTest.internal.storageClasses"),
		Prune(func(storagev1.StorageClass, *taggedStorageClass) bool { return false }),
	)
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}

	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses", `[{"name":"a","tags":["x"]},{"name":"b"}]`)
}
