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

package config

import (
	"fmt"
	"strings"
)

// cniBootstrapLookup resolves a dotted path against a nested map.
func cniBootstrapLookup(data map[string]any, path string) (any, bool) {
	path = strings.TrimPrefix(strings.TrimSpace(path), ".")
	if path == "" {
		return nil, false
	}

	parts := strings.Split(path, ".")
	var cur any = data
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, exists := m[p]
		if !exists {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// cniBootstrapMatches uses fmt.Sprint normalization so 1 matches "1" and
// true matches "true" across YAML/JSON type quirks.
func cniBootstrapMatches(value any, values []any) bool {
	if value == nil {
		return false
	}
	left := fmt.Sprint(value)
	for _, v := range values {
		if left == fmt.Sprint(v) {
			return true
		}
	}
	return false
}
