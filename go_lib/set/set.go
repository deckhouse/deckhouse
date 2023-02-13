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
	"encoding/json"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
)

// NewFromSnapshot expects snapshot to contain only strings, otherwise it panics
func NewFromSnapshot(snapshot []go_hook.FilterResult) Set {
	s := Set{}
	for _, v := range snapshot {
		if v == nil {
			continue
		}

		s.Add(v.(string))
	}
	return s
}

// NewFromValues expects values array to contain only strings, otherwise it panics
func NewFromValues(values *go_hook.PatchableValues, path string) Set {
	s := Set{}
	for _, m := range values.Get(path).Array() {
		s.Add(m.String())
	}
	return s
}

func New(xs ...string) Set {
	s := Set{}
	for _, x := range xs {
		s.Add(x)
	}
	return s
}

type Set map[string]struct{}

// Add adds strings to the set
func (s Set) Add(xs ...string) Set {
	for _, x := range xs {
		s[x] = struct{}{}
	}
	return s
}

func (s Set) AddSet(o Set) Set {
	for x := range o {
		s.Add(x)
	}
	return s
}

func (s Set) Intersection(o Set) Set {
	n := Set{}

	iterate, check := o, s
	if s.Size() > o.Size() {
		iterate = s
		check = o
	}

	for x := range iterate {
		if check.Has(x) {
			n.Add(x)
		}
	}
	return n
}

func (s Set) Delete(x string) Set {
	delete(s, x)
	return s
}

func (s Set) Has(x string) bool {
	_, ok := s[x]
	return ok
}

func (s Set) Slice() []string {
	xs := make([]string, 0, len(s))
	for x := range s {
		xs = append(xs, x)
	}
	sort.Strings(xs)
	return xs
}

func (s Set) Size() int {
	return len(s)
}

func (s Set) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Slice())
}
