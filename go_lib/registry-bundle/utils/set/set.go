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

// Set is a collection of unique elements.
type Set[T comparable] map[T]struct{}

// New returns an empty Set with optional pre-allocated capacity.
// Pre-allocating capacity improves performance when the final set size is known in advance,
// as it reduces map rehashing and memory reallocations.
//
// Example:
//
//	s := New[string]()        // creates empty set with default capacity
//	s := New[string](100)     // creates empty set pre-allocated for 100 elements
func New[T comparable](capacity ...int) Set[T] {
	if len(capacity) > 0 && capacity[0] > 0 {
		return make(Set[T], capacity[0])
	}
	return make(Set[T])
}

// Add elements to the set if not already present.
func (s Set[T]) Add(values ...T) {
	for _, v := range values {
		s[v] = struct{}{}
	}
}

// Remove elements from the set.
func (s Set[T]) Remove(values ...T) {
	for _, v := range values {
		delete(s, v)
	}
}

// Contains checks if element is in the set.
func (s Set[T]) Contains(v T) bool {
	_, ok := s[v]
	return ok
}

// Len returns the number of elements.
func (s Set[T]) Len() int {
	return len(s)
}

// Values returns all elements (order is not guaranteed).
func (s Set[T]) Values() []T {
	out := make([]T, 0, len(s))
	for v := range s {
		out = append(out, v)
	}
	return out
}
