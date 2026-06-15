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

package transformers

import (
	"encoding/json"

	"github.com/go-openapi/spec"
)

// XExtendKey is the OpenAPI extension key that signals schema inheritance.
// When present, the child schema is merged with the parent provided to Extend.
const XExtendKey = "x-extend"

// ExtendSettings holds the parsed contents of an x-extend extension block.
// Schema names the parent schema document to inherit from (reserved for future
// multi-parent support; currently the parent is passed directly to Extend).
type ExtendSettings struct {
	Schema *string `json:"schema,omitempty"`
}

// Extend is a Transformer that merges a parent schema into the child schema
// when the child carries an x-extend extension. The child's own values take
// precedence over the parent's for all merged fields.
type Extend struct {
	Parent *spec.Schema
}

// Transform merges the parent schema's definitions, extensions, required fields,
// properties, pattern properties, title, and description into s.
// A no-op when Parent is nil or x-extend is absent from s.
func (t *Extend) Transform(s *spec.Schema) *spec.Schema {
	if t.Parent == nil {
		return s
	}

	extendSettings := extractExtendSettings(s)

	if extendSettings == nil || extendSettings.Schema == nil {
		return s
	}

	// TODO check extendSettings.Schema. No need to do it for now.

	s.Definitions = mergeDefinitions(s, t.Parent)
	s.Extensions = mergeExtensions(s, t.Parent)
	s.Required = mergeRequired(s, t.Parent)
	s.Properties = mergeProperties(s, t.Parent)
	s.PatternProperties = mergePatternProperties(s, t.Parent)
	s.Title = mergeTitle(s, t.Parent)
	s.Description = mergeDescription(s, t.Parent)

	return s
}

// extractExtendSettings parses the x-extend extension value from s into an
// ExtendSettings struct. Returns nil if x-extend is absent or empty.
func extractExtendSettings(s *spec.Schema) *ExtendSettings {
	if s == nil {
		return nil
	}

	extendSettingsObj, ok := s.Extensions[XExtendKey]
	if !ok {
		return nil
	}

	if extendSettingsObj == nil {
		return nil
	}

	tmpBytes, _ := json.Marshal(extendSettingsObj)

	res := new(ExtendSettings)

	_ = json.Unmarshal(tmpBytes, res)
	return res
}

// mergeRequired returns a deduplicated union of parent.Required and s.Required,
// with parent entries listed first so parent constraints are preserved.
func mergeRequired(s *spec.Schema, parent *spec.Schema) []string {
	res := make([]string, 0)
	resIdx := make(map[string]struct{})

	for _, name := range parent.Required {
		res = append(res, name)
		resIdx[name] = struct{}{}
	}

	for _, name := range s.Required {
		if _, ok := resIdx[name]; !ok {
			res = append(res, name)
		}
	}

	return res
}

// mergeProperties returns a merged property map. Child entries override parent entries
// for the same key, so child-specific schema changes survive the merge.
func mergeProperties(s *spec.Schema, parent *spec.Schema) map[string]spec.Schema {
	res := make(map[string]spec.Schema)

	for k, v := range parent.Properties {
		res[k] = v
	}
	for k, v := range s.Properties {
		res[k] = v
	}

	return res
}

// mergePatternProperties returns a merged patternProperties map. Child entries override
// parent entries for the same pattern key.
func mergePatternProperties(s *spec.Schema, parent *spec.Schema) map[string]spec.Schema {
	res := make(map[string]spec.Schema)

	for k, v := range parent.PatternProperties {
		res[k] = v
	}
	for k, v := range s.PatternProperties {
		res[k] = v
	}

	return res
}

// mergeDefinitions returns a merged definitions map. Child entries override parent
// entries, allowing the child schema to redefine shared $ref targets.
func mergeDefinitions(s *spec.Schema, parent *spec.Schema) spec.Definitions {
	res := make(spec.Definitions)

	for k, v := range parent.Definitions {
		res[k] = v
	}
	for k, v := range s.Definitions {
		res[k] = v
	}

	return res
}

// mergeExtensions returns a merged extensions map. Child entries override parent entries.
func mergeExtensions(s *spec.Schema, parent *spec.Schema) spec.Extensions {
	ext := make(spec.Extensions)

	for k, v := range parent.Extensions {
		ext.Add(k, v)
	}
	for k, v := range s.Extensions {
		ext.Add(k, v)
	}

	return ext
}

// mergeTitle returns s.Title if non-empty, falling back to parent.Title.
func mergeTitle(s *spec.Schema, parent *spec.Schema) string {
	if s.Title != "" {
		return s.Title
	}
	return parent.Title
}

// mergeDescription returns s.Description if non-empty, falling back to parent.Description.
func mergeDescription(s *spec.Schema, parent *spec.Schema) string {
	if s.Description != "" {
		return s.Description
	}
	return parent.Description
}
