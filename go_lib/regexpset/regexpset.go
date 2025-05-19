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

package regexpset

import (
	"fmt"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
)

// NewFromSnapshot expects snapshot to contain only strings, otherwise it panics
func NewFromSnapshot(snapshot []go_hook.FilterResult) (RegExpSet, error) {
	s := RegExpSet{}
	for _, v := range snapshot {
		err := s.Add(v.(string))
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

// NewFromValues expects values array to contain only strings
func NewFromValues(values sdkpkg.PatchableValuesCollector, path string) (RegExpSet, error) {
	s := RegExpSet{}
	for _, m := range values.Get(path).Array() {
		err := s.Add(m.String())
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func New(xs ...string) (RegExpSet, error) {
	s := RegExpSet{}
	for _, x := range xs {
		err := s.Add(x)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

type RegExpSet map[string]*regexp.Regexp

// Add adds strings to the set
func (s RegExpSet) Add(xs ...string) error {
	for _, x := range xs {
		if _, ok := s[x]; ok {
			continue
		}

		r, err := regexp.Compile(x)
		if err != nil {
			return fmt.Errorf("cannot compile regexp %s: %v", x, err)
		}
		s[x] = r
	}
	return nil
}

func (s RegExpSet) Match(x string) bool {
	for _, r := range s {
		if r.MatchString(x) {
			return true
		}
	}

	return false
}
