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

package set

import (
	"sort"
	"testing"
)

// ---------------------------------------------------------------------------
// New / Add / Contains
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	s := New[int]()
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d, want 0", s.Len())
	}
}

func TestAddContains(t *testing.T) {
	tests := []struct {
		name  string
		add   []string
		check string
		want  bool
	}{
		{
			name:  "contains element after add",
			add:   []string{"a", "b"},
			check: "a",
			want:  true,
		},
		{
			name:  "does not contain element never added",
			add:   []string{"a"},
			check: "z",
			want:  false,
		},
		{
			name:  "empty set",
			add:   nil,
			check: "a",
			want:  false,
		},
		{
			name:  "duplicate add does not duplicate",
			add:   []string{"x", "x", "x"},
			check: "x",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New[string]()
			for _, v := range tt.add {
				s.Add(v)
			}
			if got := s.Contains(tt.check); got != tt.want {
				t.Errorf("Contains(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Len
// ---------------------------------------------------------------------------

func TestLen(t *testing.T) {
	tests := []struct {
		name string
		add  []int
		want int
	}{
		{
			name: "empty",
			add:  nil,
			want: 0,
		},
		{
			name: "single element",
			add:  []int{1},
			want: 1,
		},
		{
			name: "multiple unique elements",
			add:  []int{1, 2, 3},
			want: 3,
		},
		{
			name: "duplicates counted once",
			add:  []int{1, 1, 2, 2, 3},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New[int]()
			for _, v := range tt.add {
				s.Add(v)
			}
			if got := s.Len(); got != tt.want {
				t.Errorf("Len() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove(t *testing.T) {
	tests := []struct {
		name        string
		add         []string
		remove      string
		wantContain bool
		wantLen     int
	}{
		{
			name:        "remove existing element",
			add:         []string{"a", "b", "c"},
			remove:      "b",
			wantContain: false,
			wantLen:     2,
		},
		{
			name:        "remove non-existing element is no-op",
			add:         []string{"a", "b"},
			remove:      "z",
			wantContain: false,
			wantLen:     2,
		},
		{
			name:        "remove only element",
			add:         []string{"a"},
			remove:      "a",
			wantContain: false,
			wantLen:     0,
		},
		{
			name:        "remove from empty set",
			add:         nil,
			remove:      "a",
			wantContain: false,
			wantLen:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New[string]()
			for _, v := range tt.add {
				s.Add(v)
			}
			s.Remove(tt.remove)
			if got := s.Contains(tt.remove); got != tt.wantContain {
				t.Errorf("Contains(%q) after Remove = %v, want %v", tt.remove, got, tt.wantContain)
			}
			if got := s.Len(); got != tt.wantLen {
				t.Errorf("Len() after Remove = %d, want %d", got, tt.wantLen)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Values
// ---------------------------------------------------------------------------

func TestValues(t *testing.T) {
	tests := []struct {
		name string
		add  []string
		want []string
	}{
		{
			name: "empty set",
			add:  nil,
			want: []string{},
		},
		{
			name: "single element",
			add:  []string{"a"},
			want: []string{"a"},
		},
		{
			name: "multiple elements",
			add:  []string{"c", "a", "b"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "duplicates appear once",
			add:  []string{"x", "x", "y"},
			want: []string{"x", "y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New[string]()
			for _, v := range tt.add {
				s.Add(v)
			}
			got := s.Values()
			sort.Strings(got)

			want := tt.want
			if len(got) != len(want) {
				t.Fatalf("Values() len = %d, want %d; got %v", len(got), len(want), got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Errorf("Values()[%d] = %q, want %q", i, got[i], want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generic type parameter — int
// ---------------------------------------------------------------------------

func TestIntSet(t *testing.T) {
	s := New[int]()
	s.Add(1)
	s.Add(2)
	s.Add(1)

	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}
	if !s.Contains(1) {
		t.Error("Contains(1) = false, want true")
	}
	if s.Contains(99) {
		t.Error("Contains(99) = true, want false")
	}

	s.Remove(1)
	if s.Contains(1) {
		t.Error("Contains(1) after Remove = true, want false")
	}
	if s.Len() != 1 {
		t.Errorf("Len() after Remove = %d, want 1", s.Len())
	}
}
