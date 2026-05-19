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

package defaults

import (
	"github.com/flant/addon-operator/pkg/utils"
	"github.com/go-openapi/spec"
	"k8s.io/apimachinery/pkg/runtime"
)

// Apply traverses an object and apply default values from OpenAPI schema.
// It returns true if obj is changed.
//
// See https://github.com/kubernetes/kubernetes/blob/cea1d4e20b4a7886d8ff65f34c6d4f95efcb4742/staging/src/k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting/algorithm.go
//
// Note: check only Properties for object type and List validation for array type.
func Apply(obj interface{}, s *spec.Schema) bool {
	if s == nil {
		return false
	}

	res := false

	// Support utils.Values
	switch vals := obj.(type) {
	case utils.Values:
		obj = map[string]interface{}(vals)
	case *utils.Values:
		// rare case
		obj = map[string]interface{}(*vals)
	}

	switch obj := obj.(type) {
	case map[string]interface{}:
		// Apply defaults to properties
		for k, prop := range s.Properties {
			if prop.Default == nil {
				continue
			}
			if _, found := obj[k]; !found {
				obj[k] = runtime.DeepCopyJSONValue(prop.Default)
				res = true
			}
		}
		// Apply to deeper levels.
		for k, v := range obj {
			if prop, found := s.Properties[k]; found {
				deepRes := Apply(v, &prop)
				res = res || deepRes
			}
		}
	case []interface{}:
		// If the 'items' section is not specified in the schema, addon-operator will panic here.
		// The schema itself should be validated earlier before applying defaults,
		// but having a panic in runtime is much bigger problem.
		if s.Items == nil {
			return res
		}

		// Only List validation is supported.
		// See https://json-schema.org/understanding-json-schema/reference/array.html#list-validation
		for _, v := range obj {
			deepRes := Apply(v, s.Items.Schema)
			res = res || deepRes
		}
	default:
		// scalars, no action
	}

	return res
}
