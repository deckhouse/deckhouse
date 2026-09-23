/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package requirements

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	// legacyRBACv2CustomRolesRequirementKey is the release requirement key. The release.yaml of the
	// release after DKP 1.78 sets it to the maximum allowed number of legacy custom roles (0), which
	// blocks that release until every legacy-scheme custom role is migrated to the new d8:custom:*
	// scheme; the upgrade to 1.78 itself is not held back.
	legacyRBACv2CustomRolesRequirementKey = "legacyRBACv2CustomRolesCount"

	// legacyRBACv2CustomRolesValueKey mirrors hooks.LegacyRBACv2CustomRolesValueKey (the packages
	// must not import each other's internals; the contract is locked by tests).
	legacyRBACv2CustomRolesValueKey = "userAuthz:legacyRBACv2CustomRoles"

	migrationFAQReference = "see the user-authz module FAQ, section \"How do I migrate custom roles to the new scheme in DKP 1.78?\""

	// deprecatedRBACv2BindingsRequirementKey is the release requirement key of the release that
	// removes the compatibility aliases of the pre-1.78 RBACv2 role names (d8:manage:*,
	// d8:use:role:*). That release.yaml sets it to the maximum allowed number of bindings to the
	// deprecated names (0): with the aliases gone such a binding grants nothing, so the release
	// stays Pending until every binding is recreated on the new name.
	deprecatedRBACv2BindingsRequirementKey = "deprecatedRBACv2BindingsCount"
	// deprecatedRBACv2BindingsValueKey mirrors hooks.DeprecatedRBACv2BindingsValueKey (locked by
	// tests, like the pair above).
	deprecatedRBACv2BindingsValueKey = "userAuthz:deprecatedRBACv2Bindings"
	deprecatedNamesReference         = "see the user-authz module documentation, section \"Deprecated role names\", and the D8UserAuthzDeprecatedRBACv2RoleInUse / D8UserAuthzDeprecatedRBACv2CapabilityInUse alerts"
)

func init() {
	checkLegacyCustomRolesFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		allowed, err := strconv.Atoi(requirementValue)
		if err != nil {
			return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
		}

		raw, exists := getter.Get(legacyRBACv2CustomRolesValueKey)
		if !exists {
			// The discovery hook has not published a value (the module is disabled or has not synced
			// yet) — nothing to enforce.
			return true, nil
		}

		names := toStringSlice(raw)
		if len(names) <= allowed {
			return true, nil
		}

		return false, fmt.Errorf(
			"the cluster has %d custom role(s) of the legacy experimental RBACv2 scheme: %s; "+
				"they stopped aggregating permissions in DKP 1.78 — migrate them to the new d8:custom:* scheme, %s",
			len(names), strings.Join(names, ", "), migrationFAQReference)
	}

	requirements.RegisterCheck(legacyRBACv2CustomRolesRequirementKey, checkLegacyCustomRolesFunc)

	checkDeprecatedBindingsFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		allowed, err := strconv.Atoi(requirementValue)
		if err != nil {
			return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
		}
		raw, exists := getter.Get(deprecatedRBACv2BindingsValueKey)
		if !exists {
			// The hook has not published a value (the module is disabled or has not synced yet) —
			// nothing to enforce.
			return true, nil
		}
		bindings := toStringSlice(raw)
		if len(bindings) <= allowed {
			return true, nil
		}
		return false, fmt.Errorf(
			"the cluster has %d binding(s) to deprecated RBACv2 role names (d8:manage:*, d8:use:role:*, d8:use:capability:*): %s; "+
				"this release removes the compatibility aliases and such bindings would grant nothing — recreate them on the new role names, %s",
			len(bindings), strings.Join(bindings, "; "), deprecatedNamesReference)
	}
	requirements.RegisterCheck(deprecatedRBACv2BindingsRequirementKey, checkDeprecatedBindingsFunc)
}

// toStringSlice tolerates both the in-memory ([]string) and a deserialized ([]any) representation
// of the stored value.
func toStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
