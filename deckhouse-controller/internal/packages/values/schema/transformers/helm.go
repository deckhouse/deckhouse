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

// RequiredForHelm is a Transformer that promotes field names listed in the
// x-required-for-helm extension into the standard required array. This makes
// those fields mandatory at Helm rendering time even if they are optional in the
// regular values schema (e.g. fields that Helm templates always dereference).
type RequiredForHelm struct{}

// XRequiredForHelm is the OpenAPI extension key whose value is a field name or
// list of field names that must be present in the Helm values document.
const XRequiredForHelm = "x-required-for-helm"

// Transform promotes x-required-for-helm values into required for the root schema
// and recursively for all nested object properties.
func (t *RequiredForHelm) Transform(s *spec.Schema) *spec.Schema {
	s.Required = mergeRequiredFields(s.Extensions, s.Required)

	// Deep transform.
	transformRequired(s.Properties)
	return s
}

// transformRequired recursively promotes x-required-for-helm into required
// for all nested property schemas.
func transformRequired(props map[string]spec.Schema) {
	for k, prop := range props {
		prop.Required = mergeRequiredFields(prop.Extensions, prop.Required)
		props[k] = prop
		transformRequired(props[k].Properties)
	}
}

// mergeArrays returns a deduplicated union of ar1 and ar2, preserving ar1 order
// and appending unique ar2 elements at the end.
func mergeArrays(ar1 []string, ar2 []string) []string {
	res := make([]string, 0)
	m := make(map[string]struct{})
	for _, item := range ar1 {
		res = append(res, item)
		m[item] = struct{}{}
	}
	for _, item := range ar2 {
		if _, ok := m[item]; !ok {
			res = append(res, item)
		}
	}
	return res
}

// mergeRequiredFields merges field names declared in x-required-for-helm with the
// existing required slice. Supports both a single string and a []string value for
// the extension. Returns required unchanged if the extension is absent.
func mergeRequiredFields(ext spec.Extensions, required []string) []string {
	var xReqFields []string
	_, hasField := ext[XRequiredForHelm]
	if !hasField {
		return required
	}
	field, ok := ext.GetString(XRequiredForHelm)
	if ok {
		xReqFields = []string{field}
	} else {
		xReqFields, _ = ext.GetStringSlice(XRequiredForHelm)
	}

	// Merge x-required with required
	return mergeArrays(required, xReqFields)
}
