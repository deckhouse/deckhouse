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

package values

import (
	addonvalues "github.com/flant/addon-operator/pkg/utils"
)

// GrantDefault carries a runtime-resolved default value for a settings property
// located at Path (dot-separated path from the values root). It is applied by
// the grantDefaultsTransformer only when the target field is absent or holds
// an empty string.
type GrantDefault struct {
	Path  []string
	Value string
}

// applyGrantDefaults is a transformer that injects runtime-resolved cluster
// resource grant defaults into settings values. It fills each empty grantable
// field with the per-project default from the corresponding
// AvailableClusterResource.
//
// Unlike openapiDefaultsTransformer, the defaults here are dynamic — they depend
// on project-level configuration that is resolved at Application lifecycle time.
type applyGrantDefaults struct {
	defaults []GrantDefault
}

// Transform returns a copy of values with grant defaults applied to every
// property in t.defaults whose target field is absent or empty.
func (t *applyGrantDefaults) Transform(values addonvalues.Values) addonvalues.Values {
	if len(t.defaults) == 0 {
		return values
	}

	res := values.Copy()
	for _, d := range t.defaults {
		if d.Value == "" || len(d.Path) == 0 {
			continue
		}
		if !isEmptyAtPath(res, d.Path) {
			continue
		}
		setAtPath(res, d.Path, d.Value)
	}

	return res
}

// isEmptyAtPath reports whether the value at path in values is missing or an
// empty string.
func isEmptyAtPath(values map[string]interface{}, path []string) bool {
	cur := values
	for i, key := range path {
		v, ok := cur[key]
		if !ok {
			return true
		}
		if i == len(path)-1 {
			str, isStr := v.(string)
			return isStr && str == ""
		}
		next, isMap := v.(map[string]interface{})
		if !isMap {
			return false
		}
		cur = next
	}

	return true
}

// setAtPath sets value at path in values, creating intermediate maps as needed.
func setAtPath(values map[string]interface{}, path []string, value string) {
	cur := values
	for i, key := range path {
		if i == len(path)-1 {
			cur[key] = value
			return
		}
		next, ok := cur[key].(map[string]interface{})
		if !ok {
			next = map[string]interface{}{}
			cur[key] = next
		}
		cur = next
	}
}
