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

package proxy

import (
	"slices"
	"testing"
)

func TestSubjectIndex(t *testing.T) {
	t.Parallel()

	x := newSubjectIndex()
	x.add("jane-old", "jane", 1)
	janeNew := x.add("jane-new", "jane", 3)
	x.add("john", "john", 1)

	// Generation 2 revokes what is below it, of this subject only.
	if got := x.revoked("jane", 2); !slices.Equal(got, []string{"jane-old"}) {
		t.Fatalf("revoked(jane, 2) = %v, want [jane-old]", got)
	}
	if got := x.revoked("jane", 2); len(got) != 0 {
		t.Fatalf("revoked(jane, 2) again = %v, want nothing: revoked keys leave the index", got)
	}
	if got := x.len(); got != 2 {
		t.Fatalf("len() = %d, want 2", got)
	}

	// An expiry reported after the key got a new entry leaves the new entry indexed.
	newer := x.add("jane-new", "jane", 4)
	x.forget("jane-new", janeNew)
	if got := x.revoked("jane", 5); !slices.Equal(got, []string{"jane-new"}) {
		t.Fatalf("revoked(jane, 5) = %v, want [jane-new]", got)
	}
	x.forget("jane-new", newer)

	// The expiry of the current entry drops it, and an empty subject leaves no trace.
	johnSeq := x.add("john", "john", 1)
	x.forget("john", johnSeq)
	if got := x.len(); got != 0 {
		t.Fatalf("len() = %d after every entry is gone, want 0", got)
	}
	if len(x.bySubject) != 0 {
		t.Fatalf("bySubject = %v after every entry is gone, want empty", x.bySubject)
	}

	// A key that moves to another subject is found under the new one only.
	x.add("shared", "jane", 1)
	x.add("shared", "john", 1)
	if got := x.revoked("jane", 9); len(got) != 0 {
		t.Fatalf("revoked(jane, 9) = %v, want nothing", got)
	}
	if got := x.revoked("john", 9); !slices.Equal(got, []string{"shared"}) {
		t.Fatalf("revoked(john, 9) = %v, want [shared]", got)
	}
}
