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

package validation

import (
	"fmt"
	"strings"
)

func getNamedResourcePath(kind, name string) string {
	if name == "" {
		return kind
	}

	return fmt.Sprintf("%s/%s", kind, name)
}

func lookupMapStringPath(data map[string]any, path string) (string, bool) {
	if data == nil || path == "" {
		return "", false
	}

	current := any(data)
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return "", false
		}

		value, ok := object[part]
		if !ok {
			return "", false
		}

		current = value
	}

	value, ok := current.(string)
	return value, ok
}
