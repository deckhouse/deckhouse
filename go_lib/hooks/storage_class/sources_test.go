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
)

func TestStatic(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	classes, err := Static(testStorageClass{Name: "a"}, testStorageClass{Name: "b"})(context.Background(), state)
	if err != nil {
		t.Fatalf("Static() = %v", err)
	}

	assertNames(t, classes, "a", "b")
}

func TestFromExisting(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil,
		testExistingStorageClass("fast", "ssd"),
		testExistingStorageClass("slow", "hdd"),
	)
	classes, err := FromExisting(testConvertStorageClass)(context.Background(), state)
	if err != nil {
		t.Fatalf("FromExisting() = %v", err)
	}

	assertDesired(t, classes,
		testStorageClass{Name: "fast", Type: "ssd"},
		testStorageClass{Name: "slow", Type: "hdd"},
	)
}

func TestFromExistingOfAnEmptyCluster(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	classes, err := FromExisting(testConvertStorageClass)(context.Background(), state)
	if err != nil {
		t.Fatalf("FromExisting() = %v", err)
	}

	if len(classes) != 0 {
		t.Fatalf("classes = %+v, want none", classes)
	}
}

func TestFromValues(t *testing.T) {
	t.Parallel()

	values := map[string]any{
		"cloudProviderTest": map[string]any{
			"provisioned": []any{
				map[string]any{"name": "fast", "type": "ssd"},
				map[string]any{"name": "slow"},
			},
			"empty":  []any{},
			"broken": "not a list",
		},
	}

	tests := []struct {
		name    string
		path    string
		want    []testStorageClass
		wantErr bool
	}{
		{
			name: "decodes the list field by field",
			path: "cloudProviderTest.provisioned",
			want: []testStorageClass{{Name: "fast", Type: "ssd"}, {Name: "slow"}},
		},
		{
			name: "a missing path yields nothing",
			path: "cloudProviderTest.missing",
		},
		{
			name: "an empty list yields nothing",
			path: "cloudProviderTest.empty",
		},
		{
			name:    "a value that is not a list is an error",
			path:    "cloudProviderTest.broken",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state, _, _ := testState(t, values)
			classes, err := FromValues[testStorageClass](tt.path)(context.Background(), state)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("FromValues() = %+v, want an error", classes)
				}

				return
			}
			if err != nil {
				t.Fatalf("FromValues() = %v", err)
			}

			assertDesired(t, classes, tt.want...)
		})
	}
}

// The fallback of the discovering providers: the existing classes stand in until discovery arrives.
func TestFirstNonEmptyFallsBackToTheNextSource(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil, testExistingStorageClass("fast", "ssd"))
	classes, err := FirstNonEmpty(Static[testStorageClass](), FromExisting(testConvertStorageClass))(context.Background(), state)
	if err != nil {
		t.Fatalf("FirstNonEmpty() = %v", err)
	}

	assertDesired(t, classes, testStorageClass{Name: "fast", Type: "ssd"})
}

func TestFirstNonEmptyStopsAtTheFirstNonEmptySource(t *testing.T) {
	t.Parallel()

	called := false
	later := func(context.Context, *State[testStorageClass]) ([]testStorageClass, error) {
		called = true
		return nil, nil
	}

	state, _, _ := testState(t, nil)
	classes, err := FirstNonEmpty(Static(testStorageClass{Name: "discovered"}), later)(context.Background(), state)
	if err != nil {
		t.Fatalf("FirstNonEmpty() = %v", err)
	}

	assertNames(t, classes, "discovered")
	if called {
		t.Error("a source after a non-empty one was called")
	}
}

func TestFirstNonEmptyReturnsSourceErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	failing := func(context.Context, *State[testStorageClass]) ([]testStorageClass, error) { return nil, boom }

	state, _, _ := testState(t, nil)
	if _, err := FirstNonEmpty(failing, Static(testStorageClass{Name: "a"}))(context.Background(), state); !errors.Is(err, boom) {
		t.Fatalf("FirstNonEmpty() = %v, want %v", err, boom)
	}
}

func TestFirstNonEmptyOfEmptySources(t *testing.T) {
	t.Parallel()

	state, _, _ := testState(t, nil)
	classes, err := FirstNonEmpty(Static[testStorageClass](), FromExisting(testConvertStorageClass))(context.Background(), state)
	if err != nil {
		t.Fatalf("FirstNonEmpty() = %v", err)
	}

	if len(classes) != 0 {
		t.Fatalf("classes = %+v, want none", classes)
	}
}
