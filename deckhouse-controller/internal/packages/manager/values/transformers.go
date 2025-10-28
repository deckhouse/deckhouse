// Copyright 2025 Flant JSC
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

package values

import (
	addonvalues "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/addon-operator/pkg/values/validation"
	"github.com/go-openapi/spec"
)

// transformer interface for applying transformations to values.
// Implemented by types that modify values based on some logic (e.g., applying schema defaults).
type transformer interface {
	Transform(values addonvalues.Values) addonvalues.Values
}

// applyDefaults is a transformer that applies OpenAPI schema default values.
// It reads default values from an OpenAPI schema and merges them into the values.
//
// Used in the values layering process to apply:
//   - Config schema defaults (after static values, before user config)
//   - Values schema defaults (after user config)
type applyDefaults struct {
	SchemaType validation.SchemaType                  // Which schema to use (config or values)
	Schemas    map[validation.SchemaType]*spec.Schema // Available OpenAPI schemas
}

// Transform applies default values from the OpenAPI schema to the provided values.
// Returns the input values unchanged if no schema is available.
//
// Process:
//  1. Check if schemas are loaded
//  2. Get the schema for the specified type
//  3. Make a copy of values to avoid mutations
//  4. Apply defaults from schema using validation package
//
// Returns a new values object with defaults applied (original values are not modified).
func (a *applyDefaults) Transform(values addonvalues.Values) addonvalues.Values {
	// Return unchanged if no schemas loaded
	if a.Schemas == nil {
		return values
	}

	// Get the schema for our type (config or values)
	schema := a.Schemas[a.SchemaType]
	if schema == nil {
		return values
	}

	// Make a copy to avoid mutating the input
	res := values.Copy()

	// Apply default values from the schema
	validation.ApplyDefaults(res, schema)

	return res
}
