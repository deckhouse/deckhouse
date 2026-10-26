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
	"reflect"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type testStorageClass struct {
	Name string
	Type string
}

func (sc testStorageClass) GetName() string { return sc.Name }

// fakePatchCollector records the StorageClasses deleted; nothing else is expected of it.
type fakePatchCollector struct {
	go_hook.PatchCollector
	deleted []string
}

func (c *fakePatchCollector) Delete(_, _, _, name string) {
	c.deleted = append(c.deleted, name)
}

func testObject(name, classType string) storagev1.StorageClass {
	return storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Parameters: map[string]string{"type": classType},
	}
}

func testObjectToDesired(object storagev1.StorageClass) testStorageClass {
	return testStorageClass{Name: object.Name, Type: object.Parameters["type"]}
}

func TestApplyFilter(t *testing.T) {
	t.Parallel()

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  "storage.k8s.io/v1",
		"kind":        "StorageClass",
		"metadata":    map[string]any{"name": "fast"},
		"provisioner": "csi.example.com",
		"parameters":  map[string]any{"type": "ssd"},
	}}

	result, err := ApplyFilter(obj)
	if err != nil {
		t.Fatalf("ApplyFilter() error = %v", err)
	}

	storageClass, ok := result.(*storagev1.StorageClass)
	if !ok {
		t.Fatalf("ApplyFilter() = %T, want *storagev1.StorageClass", result)
	}
	if storageClass.Name != "fast" || storageClass.Provisioner != "csi.example.com" || storageClass.Parameters["type"] != "ssd" {
		t.Errorf("ApplyFilter() = %+v", storageClass)
	}
}

func TestExcludeByAnchoredPatterns(t *testing.T) {
	t.Parallel()

	classes := []testStorageClass{{Name: "fast"}, {Name: "ultra-fast-ssd"}, {Name: "network-hdd"}, {Name: "network-ssd"}}

	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{name: "no patterns exclude nothing", patterns: nil, want: []string{"fast", "ultra-fast-ssd", "network-hdd", "network-ssd"}},
		{name: "a plain name matches the whole name only", patterns: []string{"fast"}, want: []string{"ultra-fast-ssd", "network-hdd", "network-ssd"}},
		{name: "a pattern matches the whole name", patterns: []string{".*-ssd"}, want: []string{"fast", "network-hdd"}},
		{name: "alternation stays inside the anchors", patterns: []string{"fast|network-hdd"}, want: []string{"ultra-fast-ssd", "network-ssd"}},
		{name: "everything excluded", patterns: []string{".*"}, want: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ExcludeByAnchoredPatterns(classes, tt.patterns)
			if err != nil {
				t.Fatalf("ExcludeByAnchoredPatterns() error = %v", err)
			}
			if got == nil {
				t.Fatalf("ExcludeByAnchoredPatterns() = nil, want a non-nil slice")
			}
			assertNames(t, got, tt.want...)
		})
	}
}

func TestExcludeByAnchoredPatternsRejectsAnInvalidPattern(t *testing.T) {
	t.Parallel()

	if _, err := ExcludeByAnchoredPatterns([]testStorageClass{{Name: "a"}}, []string{"([a-z"}); err == nil {
		t.Fatal("ExcludeByAnchoredPatterns() error = nil, want an error")
	}
}

func TestMergeOverridingByName(t *testing.T) {
	t.Parallel()

	base := []testStorageClass{
		{Name: "network-hdd", Type: "network-hdd"},
		{Name: "network-ssd", Type: "network-ssd"},
		{Name: "network-ssd-nonreplicated", Type: "network-ssd-nonreplicated"},
	}
	overrides := []testStorageClass{
		{Name: "network-ssd-64k", Type: "first"},
		{Name: "network-ssd", Type: "override"},
		{Name: "network-ssd-64k", Type: "last"},
	}

	got := MergeOverridingByName(base, overrides)

	want := []testStorageClass{
		{Name: "network-hdd", Type: "network-hdd"},
		{Name: "network-ssd", Type: "override"},
		{Name: "network-ssd-nonreplicated", Type: "network-ssd-nonreplicated"},
		{Name: "network-ssd-64k", Type: "last"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeOverridingByName() = %+v, want %+v", got, want)
	}

	if base[1].Type != "network-ssd" {
		t.Errorf("MergeOverridingByName() changed base: %+v", base)
	}
}

func TestSortByNameIsStable(t *testing.T) {
	t.Parallel()

	classes := []testStorageClass{{Name: "b"}, {Name: "a", Type: "first"}, {Name: "a", Type: "second"}}
	SortByName(classes)

	want := []testStorageClass{{Name: "a", Type: "first"}, {Name: "a", Type: "second"}, {Name: "b"}}
	if !reflect.DeepEqual(classes, want) {
		t.Errorf("SortByName() = %+v, want %+v", classes, want)
	}
}

// Only an object that is still desired and differs from its entry is deleted; one the module no longer
// wants is not this function's to remove.
func TestDeleteChangedObjects(t *testing.T) {
	t.Parallel()

	patches := &fakePatchCollector{}
	input := &go_hook.HookInput{Logger: log.NewNop(), PatchCollector: patches}

	objects := []storagev1.StorageClass{
		testObject("unchanged", "ssd"),
		testObject("changed", "hdd"),
		testObject("not-desired", "hdd"),
	}
	desired := []testStorageClass{
		{Name: "unchanged", Type: "ssd"},
		{Name: "changed", Type: "ssd"},
		{Name: "new", Type: "ssd"},
	}

	DeleteChangedObjects(input, objects, desired, testObjectToDesired)

	if !reflect.DeepEqual(patches.deleted, []string{"changed"}) {
		t.Errorf("deleted = %v, want [changed]", patches.deleted)
	}
}

func TestDeleteObjectsIf(t *testing.T) {
	t.Parallel()

	patches := &fakePatchCollector{}
	input := &go_hook.HookInput{Logger: log.NewNop(), PatchCollector: patches}

	objects := []storagev1.StorageClass{testObject("a", "hdd"), testObject("b", "ssd"), testObject("c", "hdd")}

	DeleteObjectsIf(input, objects, func(object storagev1.StorageClass) bool {
		return object.Parameters["type"] == "hdd"
	})

	if !reflect.DeepEqual(patches.deleted, []string{"a", "c"}) {
		t.Errorf("deleted = %v, want [a c]", patches.deleted)
	}
}

func assertNames(t *testing.T, classes []testStorageClass, want ...string) {
	t.Helper()

	got := make([]string, 0, len(classes))
	for _, class := range classes {
		got = append(got, class.Name)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
}
