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

package crdenricher

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

// childMap returns the nested mapping stored under key, or nil when it is
// absent or not a mapping. Using sigs.k8s.io/yaml means every mapping decodes
// to a map[string]any with string keys.
func childMap(node map[string]any, key string) map[string]any {
	if node == nil {
		return nil
	}
	if child, ok := node[key].(map[string]any); ok {
		return child
	}
	return nil
}

// decodeValue parses a marker value as YAML, yielding scalars, lists or maps
// that mirror the representation used by the rest of the document.
func decodeValue(raw string) (any, error) {
	var out any
	if err := yaml.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode value %q: %w", raw, err)
	}
	return out, nil
}
