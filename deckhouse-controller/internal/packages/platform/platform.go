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

// Package platform builds the values exposed to helm templates under the
// .Platform prefix. .Platform is the new contract that replaces the legacy
// addon-operator global values: any value reachable as global.<path> is also
// reachable as .Platform.<path>.
package platform

import (
	addonutils "github.com/flant/addon-operator/pkg/utils"
)

const (
	enabledModulesKey = "enabledModules"

	// EnabledModulesKey exposes the enabled modules list under .Platform.EnabledModules.
	EnabledModulesKey = "EnabledModules"
	// CapabilitiesKey exposes platform capabilities under .Platform.Capabilities.
	CapabilitiesKey = "Capabilities"
	// CapabilitiesHasKey lists enabled capabilities under .Platform.Capabilities.Has.
	CapabilitiesHasKey = "Has"
)

// BuildValues builds the values exposed to helm templates under the .Platform
// prefix. It mirrors the entire global values tree (so that any global.<path>
// is reachable as .Platform.<path>) and augments it with structured platform
// fields that are not part of the global values:
//
//   - EnabledModules: the enabled modules list (PascalCase mirror of
//     global.enabledModules for the new .Platform contract).
//   - Capabilities.Has: the CRD GVKs (group/version/kind) currently served by
//     enabled modules. A nil/empty capabilities slice yields an empty list.
func BuildValues(global addonutils.Values, capabilities []string) addonutils.Values {
	platform := make(addonutils.Values, len(global)+2)

	// Mirror the full global values tree as-is (camelCase keys preserved),
	// so global.<path> resolves identically as .Platform.<path>.
	for key, value := range global {
		platform[key] = value
	}

	has := capabilities
	if has == nil {
		has = []string{}
	}

	platform[EnabledModulesKey] = enabledModulesFrom(global)
	platform[CapabilitiesKey] = addonutils.Values{
		CapabilitiesHasKey: has,
	}

	return platform
}

// enabledModulesFrom extracts the enabledModules list from global values,
// tolerating both []string and []interface{} representations. Returns an empty
// slice when the key is absent or has an unexpected type.
func enabledModulesFrom(global addonutils.Values) []string {
	raw, ok := global[enabledModulesKey]
	if !ok {
		return []string{}
	}

	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return []string{}
	}
}
