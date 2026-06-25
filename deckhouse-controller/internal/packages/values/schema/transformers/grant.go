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
	"github.com/go-openapi/spec"
)

// XGrant is the OpenAPI extension key that binds a settings field to a grantable
// cluster resource (see multitenancy-manager AvailableClusterResource).
const XGrant = "x-deckhouse-grantable-resource"

// RequiredForGrant is a Transformer that promotes fields tagged with
// x-deckhouse-grantable-resource into the parent's required array, making
// them mandatory. This ensures the user must explicitly provide a value
// instead of relying on silent default injection.
type RequiredForGrant struct{}

// Transform adds property names that carry the x-deckhouse-grantable-resource
// extension to the required array of the containing object schema.
func (t *RequiredForGrant) Transform(s *spec.Schema) *spec.Schema {
	s.Required = mergeGrantRequired(s.Properties, s.Required)
	transformGrantRequired(s.Properties)
	return s
}

func transformGrantRequired(props map[string]spec.Schema) {
	for k, prop := range props {
		prop.Required = mergeGrantRequired(prop.Properties, prop.Required)
		props[k] = prop
		transformGrantRequired(prop.Properties)
	}
}

func mergeGrantRequired(props map[string]spec.Schema, required []string) []string {
	var grantFields []string
	for name, prop := range props {
		if _, ok := prop.Extensions[XGrant]; ok {
			grantFields = append(grantFields, name)
		}
	}
	return mergeArrays(required, grantFields)
}
