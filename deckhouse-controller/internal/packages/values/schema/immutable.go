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
	"regexp"

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
	walkImmutable(s, s, oldValues, newValues, nil, &errs)

	return errs
}

// walkImmutable descends the schema and both value trees in lockstep. root is the schema
// the walk started from, the only place a $ref can be looked up.
func walkImmutable(root, s *spec.Schema, oldValue, newValue any, path []string, errs *[]error) {
	// Two absent values cannot differ anywhere below. This is also what ends the descent
	// when a recursive $ref keeps handing back the same schema.
	if oldValue == nil && newValue == nil {
		return
	}

	// Loaders expand $ref in place, but a recursive definition is deliberately left
	// unexpanded: resolve it here or everything below it goes unchecked. Resolution is a
	// JSON pointer lookup inside root, so it never fetches a remote document.
	if s.Ref.String() != "" {
		resolved, err := spec.ResolveRef(root, &s.Ref)
		if err != nil || resolved == nil {
			return
		}

		s = resolved
	}

	if immutable, ok := s.Extensions.GetBool(XImmutable); ok && immutable {
		if err := compareImmutable(path, oldValue, newValue); err != nil {
			*errs = append(*errs, err)
		}

		return
	}

	walkSubschemas(root, s, oldValue, newValue, path, errs)

	oldMap, oldIsMap := oldValue.(map[string]any)
	newMap, newIsMap := newValue.(map[string]any)
	if oldIsMap || newIsMap {
		for name := range s.Properties {
			prop := s.Properties[name]
			// Indexing a nil map is fine: a key missing on one side yields nil there,
			// which is exactly what compareImmutable needs to see.
			walkImmutable(root, &prop, oldMap[name], newMap[name], childPath(path, name), errs)
		}

		// Keys the schema does not name are governed by patternProperties, or by
		// additionalProperties when no pattern claims them.
		for _, key := range unionKeys(oldMap, newMap) {
			if _, named := s.Properties[key]; named {
				continue
			}

			if sub := unnamedSchema(s, key); sub != nil {
				walkImmutable(root, sub, oldMap[key], newMap[key], childPath(path, key), errs)
			}
		}
	}

	oldList, oldIsList := oldValue.([]any)
	newList, newIsList := newValue.([]any)
	if oldIsList || newIsList {
		// Elements pair up by index, and the walk runs to the longer side so that a
		// truncated list compares its dropped tail instead of leaving it unchecked.
		for i := 0; i < max(len(oldList), len(newList)); i++ {
			if sub := itemSchema(s, i); sub != nil {
				walkImmutable(root, sub, itemValue(oldList, i), itemValue(newList, i), childPath(path, fmt.Sprintf("[%d]", i)), errs)
			}
		}
	}
}

// walkSubschemas descends the keywords that constrain the same value at the same path, so
// they add no path segment: a mark inside one of them applies to the field it wraps.
func walkSubschemas(root, s *spec.Schema, oldValue, newValue any, path []string, errs *[]error) {
	for _, branches := range [][]spec.Schema{s.AllOf, s.AnyOf, s.OneOf} {
		for i := range branches {
			walkImmutable(root, &branches[i], oldValue, newValue, path, errs)
		}
	}

	if s.Not != nil {
		walkImmutable(root, s.Not, oldValue, newValue, path, errs)
	}

	for name := range s.Dependencies {
		if dep := s.Dependencies[name].Schema; dep != nil {
			walkImmutable(root, dep, oldValue, newValue, path, errs)
		}
	}
}

// unionKeys lists every key present on either side, old ones first.
func unionKeys(oldMap, newMap map[string]any) []string {
	keys := make([]string, 0, len(oldMap)+len(newMap))
	for key := range oldMap {
		keys = append(keys, key)
	}

	for key := range newMap {
		if _, ok := oldMap[key]; !ok {
			keys = append(keys, key)
		}
	}

	return keys
}

// unnamedSchema returns the schema governing a key that properties does not list: the
// first patternProperties entry whose regexp matches it, otherwise additionalProperties.
func unnamedSchema(s *spec.Schema, key string) *spec.Schema {
	for pattern := range s.PatternProperties {
		re, err := regexp.Compile(pattern)
		if err != nil || !re.MatchString(key) {
			continue
		}

		sub := s.PatternProperties[pattern]

		return &sub
	}

	if s.AdditionalProperties != nil {
		return s.AdditionalProperties.Schema
	}

	return nil
}

// itemSchema returns the schema for the element at index i: the single list-validation
// schema, or the tuple entry at that position with additionalItems covering the rest.
func itemSchema(s *spec.Schema, i int) *spec.Schema {
	if s.Items == nil {
		return nil
	}

	if s.Items.Schema != nil {
		return s.Items.Schema
	}

	if i < len(s.Items.Schemas) {
		return &s.Items.Schemas[i]
	}

	if s.AdditionalItems != nil {
		return s.AdditionalItems.Schema
	}

	return nil
}

// itemValue returns the element at index i, or nil when the list is shorter than that.
func itemValue(list []any, i int) any {
	if i >= len(list) {
		return nil
	}

	return list[i]
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
