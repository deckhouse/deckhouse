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

package provider

import "fmt"

func extractSetting[T any](key string, settings map[string]any, providerName string) (T, error) {
	var zero T

	vRaw, ok := settings[key]
	if !ok {
		return zero, fmt.Errorf("key %s not found in provider %s settings", key, providerName)
	}

	v, ok := vRaw.(T)
	if !ok {
		return zero, fmt.Errorf("key %s is not a string in provider %s settings", key, providerName)
	}

	return v, nil
}

func extractString(key string, settings map[string]any, providerName string) (string, error) {
	return extractSetting[string](key, settings, providerName)
}
