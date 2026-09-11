// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schema

import (
	"fmt"
	"reflect"

	"github.com/go-openapi/spec"
)

// XImmutable is the OpenAPI extension key that freezes a settings field once the
// application is created: the value may be chosen at install time and never changed afterwards.
const XImmutable = "x-deckhouse-immutable"

// CheckImmutable compares the stored settings against the incoming ones and returns
// one error per field marked x-deckhouse-immutable whose value changed.
func CheckImmutable(s *spec.Schema, oldValues, newValues map[string]any) []error {
	if s == nil {
		return nil
	}

	var errs []error
	walkImmutable(s, oldValues, newValues, nil, &errs)

	return errs
}

// walkImmutable descends the schema and both value trees in lockstep.
func walkImmutable(s *spec.Schema, oldValue, newValue any, path []string, errs *[]error) {
	if immutable, ok := s.Extensions.GetBool(XImmutable); ok && immutable {
		if err := compareImmutable(path, oldValue, newValue); err != nil {
			*errs = append(*errs, err)
		}

		return
	}

	// allOf / anyOf / oneOf branches constrain the same value at the same path, so they add no path segment.
	for _, branches := range [][]spec.Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for i := range branches {
			walkImmutable(&branches[i], oldValue, newValue, path, errs)
		}
	}

	oldMap, oldIsMap := oldValue.(map[string]any)
	newMap, newIsMap := newValue.(map[string]any)
	if oldIsMap || newIsMap {
		for name := range s.Properties {
			prop := s.Properties[name]
			// Indexing a nil map is fine: a key missing on one side yields nil there,
			// which is exactly what compareImmutable needs to see.
			walkImmutable(&prop, oldMap[name], newMap[name], childPath(path, name), errs)
		}

		// additionalProperties entries sit under keys the schema does not name, so only
		// keys present on both sides can be compared.
		if ap := s.AdditionalProperties; ap != nil && ap.Schema != nil {
			for key := range oldMap {
				if _, ok := newMap[key]; !ok {
					continue
				}

				walkImmutable(ap.Schema, oldMap[key], newMap[key], childPath(path, key), errs)
			}
		}
	}

	// Only list validation is supported, the same subset defaults.Apply handles.
	if s.Items != nil && s.Items.Schema != nil {
		oldList, _ := oldValue.([]any)
		newList, _ := newValue.([]any)

		for i := 0; i < len(oldList) && i < len(newList); i++ {
			walkImmutable(s.Items.Schema, oldList[i], newList[i], childPath(path, fmt.Sprintf("[%d]", i)), errs)
		}
	}
}

// childPath appends a segment to path on a fresh slice. Reusing the backing array
// would let sibling properties overwrite each other's path.
func childPath(path []string, segment string) []string {
	return append(append([]string(nil), path...), segment)
}

// compareImmutable decides whether an immutable field changed between the stored and
// the incoming settings. Both values arrive with schema defaults already applied, and
// either side is nil when the key is absent there. Returns nil when the transition is
// allowed, an error naming joinPath(path) when it is not.
func compareImmutable(path []string, oldValue, newValue any) error {
	// No stored value means the field was never set: choosing one now is the install-time
	// choice happening late, not an edit of a value the application already runs with.
	// Defaults are applied before the comparison, so this only covers fields that have
	// no default either.
	if oldValue == nil {
		return nil
	}

	// DeepEqual takes nested maps and slices whole, which is what a mark on an object
	// needs: the block is one value. Both sides come from the same JSON decoding, so
	// numbers are float64 on both and the comparison never trips over types alone.
	if reflect.DeepEqual(oldValue, newValue) {
		return nil
	}

	return fmt.Errorf("field %q is immutable: it can only be set when the application is created", joinPath(path))
}
