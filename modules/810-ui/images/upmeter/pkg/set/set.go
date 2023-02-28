/*
Copyright 2023 Flant JSC

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
)

func New(xs ...string) StringSet {
	s := StringSet{}
	for _, x := range xs {
		s.Add(x)
	}
	return s
}

type StringSet map[string]struct{}

// Add adds strings to the set
func (s StringSet) Add(xs ...string) StringSet {
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

func (s StringSet) AddSet(o StringSet) StringSet {
	for x := range o {
		s.Add(x)
	}
	return s
}

func (s StringSet) Delete(x string) StringSet {
	delete(s, x)
	return s
}

func (s StringSet) Has(x string) bool {
	_, ok := s[x]
	return ok
}

func (s StringSet) Slice() []string {
	xs := make([]string, 0, len(s))
	for x := range s {
		xs = append(xs, x)
	}
	sort.Strings(xs)
	return xs
}

func (s StringSet) Size() int {
	return len(s)
}
