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

	storagev1 "k8s.io/api/storage/v1"
)

func testInternalValues() map[string]any {
	return map[string]any{"cloudProviderTest": map[string]any{"internal": map[string]any{}}}
}

func TestPublish(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, testInternalValues())
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "b", Type: "hdd"}, testStorageClass{Name: "a"})),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
	)

	// Publish writes the set as it is; ordering is SortByName's job.
	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses", `[{"name":"b","type":"hdd"},{"name":"a"}]`)
}

// Publish writes an empty set as an empty list rather than null; a module that wants to keep the
// previous value puts StopIfEmpty in front of it.
func TestPublishWritesAnEmptyList(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, testInternalValues())
	mustRun(t, state, Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"))

	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses", `[]`)
}

// Every class may have been excluded: the empty result is still published, so the templates stop
// rendering the excluded classes.
func TestPublishAfterEverythingIsExcluded(t *testing.T) {
	t.Parallel()

	state, values, _ := testState(t, map[string]any{
		"cloudProviderTest": map[string]any{"internal": map[string]any{}, "exclude": []any{".*"}},
	})
	mustRun(t, state,
		Append(Static(testStorageClass{Name: "a"})),
		Exclude[testStorageClass]("cloudProviderTest.exclude"),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
	)

	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses", `[]`)
}

func TestPublishDefault(t *testing.T) {
	t.Parallel()

	isDefault := func(sc testStorageClass) bool { return sc.Type == "default" }

	tests := []struct {
		name    string
		values  map[string]any
		desired []testStorageClass
		op      string
		value   string
	}{
		{
			name:    "the class the predicate accepts",
			values:  testInternalValues(),
			desired: []testStorageClass{{Name: "fast"}, {Name: "slow", Type: "default"}},
			op:      "add",
			value:   `"slow"`,
		},
		{
			// The first match wins, so the order the earlier steps left is what decides.
			name:    "the first of several defaults",
			values:  testInternalValues(),
			desired: []testStorageClass{{Name: "b", Type: "default"}, {Name: "a", Type: "default"}},
			op:      "add",
			value:   `"b"`,
		},
		{
			// A stale value would keep pointing at a class that may have just been excluded.
			name: "no default removes a stale value",
			values: map[string]any{
				"cloudProviderTest": map[string]any{"internal": map[string]any{"defaultStorageClass": "gone"}},
			},
			desired: []testStorageClass{{Name: "fast"}},
			op:      "remove",
		},
		{
			name: "an empty set removes a stale value",
			values: map[string]any{
				"cloudProviderTest": map[string]any{"internal": map[string]any{"defaultStorageClass": "gone"}},
			},
			op: "remove",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state, values, _ := testState(t, tt.values)
			state.Desired = tt.desired
			mustRun(t, state, PublishDefault(isDefault, "cloudProviderTest.internal.defaultStorageClass"))

			assertPatch(t, values, tt.op, "/cloudProviderTest/internal/defaultStorageClass", tt.value)
		})
	}
}

func TestPrunePassesTheWantedEntry(t *testing.T) {
	t.Parallel()

	seen := map[string]*testStorageClass{}
	state, _, patches := testState(t, nil,
		testExistingStorageClass("wanted", "ssd"),
		testExistingStorageClass("unwanted", "hdd"),
	)
	state.Desired = []testStorageClass{{Name: "wanted", Type: "ssd"}}
	mustRun(t, state, Prune(func(actual storagev1.StorageClass, desired *testStorageClass) bool {
		seen[actual.Name] = desired
		return desired == nil
	}))

	if desired := seen["wanted"]; desired == nil || *desired != (testStorageClass{Name: "wanted", Type: "ssd"}) {
		t.Errorf("desired for a wanted class = %+v, want the desired entry", desired)
	}
	if desired, found := seen["unwanted"]; !found || desired != nil {
		t.Errorf("desired for an unwanted class = %+v (seen=%v), want nil", desired, found)
	}
	if len(patches.deleted) != 1 || patches.deleted[0] != "unwanted" {
		t.Fatalf("deleted = %v, want [unwanted]", patches.deleted)
	}
}

// The pointer handed to the predicate is a copy per class: keeping it must not alias the next one.
func TestPrunePassesDistinctEntries(t *testing.T) {
	t.Parallel()

	var kept []*testStorageClass
	state, _, _ := testState(t, nil,
		testExistingStorageClass("a", "ssd"),
		testExistingStorageClass("b", "hdd"),
	)
	state.Desired = []testStorageClass{{Name: "a", Type: "ssd"}, {Name: "b", Type: "hdd"}}
	mustRun(t, state, Prune(func(_ storagev1.StorageClass, desired *testStorageClass) bool {
		kept = append(kept, desired)
		return false
	}))

	if len(kept) != 2 || kept[0].Name != "a" || kept[1].Name != "b" {
		t.Fatalf("kept = %+v, want a then b", kept)
	}
}

func TestPruneOfAnEmptyCluster(t *testing.T) {
	t.Parallel()

	state, _, patches := testState(t, nil)
	state.Desired = []testStorageClass{{Name: "a"}}
	mustRun(t, state, Prune(func(storagev1.StorageClass, *testStorageClass) bool { return true }))

	if len(patches.deleted) != 0 {
		t.Fatalf("deleted = %v, want none", patches.deleted)
	}
}

func TestPruneModified(t *testing.T) {
	t.Parallel()

	state, _, patches := testState(t, nil,
		testExistingStorageClass("unchanged", "ssd"),
		testExistingStorageClass("changed", "hdd"),
		testExistingStorageClass("legacy", "hdd"),
	)
	state.Desired = []testStorageClass{{Name: "unchanged", Type: "ssd"}, {Name: "changed", Type: "ssd"}}
	mustRun(t, state, PruneModified(testConvertStorageClass))

	if len(patches.deleted) != 1 || patches.deleted[0] != "changed" {
		t.Fatalf("deleted = %v, want [changed]", patches.deleted)
	}
}

func TestModifiedPredictor(t *testing.T) {
	t.Parallel()

	shouldPrune := ModifiedPredictor(testConvertStorageClass)

	tests := []struct {
		name    string
		actual  storagev1.StorageClass
		desired *testStorageClass
		want    bool
	}{
		{
			name:    "an unchanged class stays",
			actual:  testExistingStorageClass("fast", "ssd"),
			desired: &testStorageClass{Name: "fast", Type: "ssd"},
		},
		{
			name:    "a class with changed parameters goes",
			actual:  testExistingStorageClass("fast", "hdd"),
			desired: &testStorageClass{Name: "fast", Type: "ssd"},
			want:    true,
		},
		{
			// Deleting it would take away storage the operator may still be using; it is only
			// gone from the desired list, not from the cluster.
			name:    "a class the module no longer wants stays",
			actual:  testExistingStorageClass("legacy", "hdd"),
			desired: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shouldPrune(tt.actual, tt.desired); got != tt.want {
				t.Errorf("shouldPrune() = %v, want %v", got, tt.want)
			}
		})
	}
}

// The whole pipeline of Yandex Cloud: defaults, provisioned overrides and additions, exclusion after
// that, publication in name order, and recreation of a provisioned class whose parameters changed.
func TestProvisionedStorageClassesPipeline(t *testing.T) {
	t.Parallel()

	state, values, patches := testState(t, map[string]any{
		"cloudProviderTest": map[string]any{
			"internal": map[string]any{},
			"provisioned": []any{
				map[string]any{"name": "network-ssd", "type": "network-ssd-64k"},
				map[string]any{"name": "network-ssd-io", "type": "network-ssd-io-m3"},
				map[string]any{"name": "network-hdd-big", "type": "network-hdd"},
			},
			"exclude": []any{".*-big"},
		},
	},
		testExistingStorageClass("network-ssd", "network-ssd"),
		testExistingStorageClass("network-hdd", "network-hdd"),
	)
	mustRun(t, state,
		Append(Static(
			testStorageClass{Name: "network-ssd", Type: "network-ssd"},
			testStorageClass{Name: "network-hdd", Type: "network-hdd"},
		)),
		OverrideByName(FromValues[testStorageClass]("cloudProviderTest.provisioned")),
		Exclude[testStorageClass]("cloudProviderTest.exclude"),
		SortByName[testStorageClass](),
		Publish[testStorageClass]("cloudProviderTest.internal.storageClasses"),
		PruneModified(testConvertStorageClass),
	)

	assertPatch(t, values, "add", "/cloudProviderTest/internal/storageClasses",
		`[{"name":"network-hdd","type":"network-hdd"},{"name":"network-ssd","type":"network-ssd-64k"},{"name":"network-ssd-io","type":"network-ssd-io-m3"}]`)
	if len(patches.deleted) != 1 || patches.deleted[0] != "network-ssd" {
		t.Fatalf("deleted = %v, want [network-ssd]", patches.deleted)
	}
}
