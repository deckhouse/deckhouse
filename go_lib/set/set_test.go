/*
Copyright 2021 Flant JSC

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

package set

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewAndHas(t *testing.T) {
	tests := []struct {
		name   string
		items  []string
		assert map[string]bool
	}{
		{
			name: "empty",
			assert: map[string]bool{
				"any": false,
			},
		},
		{
			name:  "single item",
			items: []string{"zz"},
			assert: map[string]bool{
				"zz": true,
				"xx": false,
			},
		},
		{
			name:  "three items",
			items: []string{"xx", "zz", "aa"},
			assert: map[string]bool{
				"zz": true,
				"xx": true,
				"aa": true,
				"ww": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.items...)

			for x, shouldBe := range tt.assert {
				if shouldBe && !s.Has(x) {
					t.Errorf("missing expected item %s", x)
				}

				if !shouldBe && s.Has(x) {
					t.Errorf("unexpected item %s", x)
				}
			}
		})
	}
}

func Test_AddAndHas(t *testing.T) {
	tests := []struct {
		name   string
		items  []string
		assert map[string]bool
	}{
		{
			name: "empty",
			assert: map[string]bool{
				"any": false,
			},
		},
		{
			name:  "single item",
			items: []string{"zz"},
			assert: map[string]bool{
				"zz": true,
				"xx": false,
			},
		},
		{
			name:  "three items",
			items: []string{"xx", "zz", "aa"},
			assert: map[string]bool{
				"zz": true,
				"xx": true,
				"aa": true,
				"ww": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			s.Add(tt.items...)

			for x, shouldBe := range tt.assert {
				if shouldBe && !s.Has(x) {
					t.Errorf("missing expected item %s", x)
				}

				if !shouldBe && s.Has(x) {
					t.Errorf("unexpected item %s", x)
				}
			}
		})
	}
}

func Test_AddSet(t *testing.T) {
	tests := []struct {
		name      string
		initial   Set
		added     Set
		uniqItems []string
	}{
		{
			name:      "both filled",
			initial:   New("a", "b", "c"),
			added:     New("z", "b", "x"),
			uniqItems: []string{"a", "b", "c", "x", "z"},
		},
		{
			name:      "initial is filled",
			initial:   New("a", "b", "c"),
			added:     New(),
			uniqItems: []string{"a", "b", "c"},
		},
		{
			name:      "added is filled",
			initial:   New(),
			added:     New("a", "b", "c"),
			uniqItems: []string{"a", "b", "c"},
		},
		{
			name:      "both empty",
			initial:   New(),
			added:     New(),
			uniqItems: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.initial.AddSet(tt.added)

			for _, x := range tt.uniqItems {
				if !tt.initial.Has(x) {
					t.Errorf("missing expected item %s", x)
				}
				if len(tt.initial) != len(tt.uniqItems) {
					t.Errorf("unexpected set zise: want=%d, got=%d", len(tt.uniqItems), len(tt.initial))
				}
			}
		})
	}
}

func Test_Delete(t *testing.T) {
	{
		// deletion actually works
		s := New()
		s.Add("")
		s.Delete("")
		if s.Size() > 0 {
			t.Errorf("expected empty set")
		}
	}

	{
		// deletion ignores absent items
		s := New()
		s.Delete("")
		if s.Size() > 0 {
			t.Errorf("expected empty set")
		}
	}
}

func Test_Slice(t *testing.T) {
	// the slice is sorted
	s := New("x", "z", "1", "f", "a", "g", "n")
	expected := []string{"1", "a", "f", "g", "n", "x", "z"}
	assert.Equal(t, expected, s.Slice(), "Slice() must sort strings")
}
