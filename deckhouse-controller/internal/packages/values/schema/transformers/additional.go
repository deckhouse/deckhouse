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

import "github.com/go-openapi/spec"

// AdditionalProperties is a Transformer that sets AdditionalProperties to false on
// every object schema node that does not already define it. This prevents values
// documents from containing undeclared keys that would otherwise pass validation silently.
type AdditionalProperties struct {
	Parent *spec.Schema
}

// Transform sets AdditionalProperties to false on s and recursively on every
// nested property and array item schema that has not already constrained it.
func (t *AdditionalProperties) Transform(s *spec.Schema) *spec.Schema {
	if s.AdditionalProperties == nil {
		s.AdditionalProperties = &spec.SchemaOrBool{
			Allows: false,
		}
	}

	for k, prop := range s.Properties {
		if prop.AdditionalProperties == nil {
			prop.AdditionalProperties = &spec.SchemaOrBool{
				Allows: false,
			}
			ts := prop
			s.Properties[k] = *t.Transform(&ts)
		}
	}

	if s.Items != nil {
		if s.Items.Schema != nil {
			s.Items.Schema = t.Transform(s.Items.Schema)
		}
		for i, item := range s.Items.Schemas {
			ts := item
			s.Items.Schemas[i] = *t.Transform(&ts)
		}
	}

	return s
}
