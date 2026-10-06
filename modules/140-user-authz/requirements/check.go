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
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks"
)

const (
	// legacyRBACv2CustomRolesRequirementKey is the release requirement key. Its value is the maximum
	// number of custom roles that have lost access the role model gave them, because their
	// aggregationRule selects capabilities by the rbac.deckhouse.io/kind manage or use label or by
	// the aggregation label of a lineage that no built-in role collects and no custom capability
	// carries, or because they have rbac.deckhouse.io/use-role without the system or subsystem scope
	// and get no RoleBindings in the namespaces of the modules. A release that carries the key waits
	// while the cluster has more of them.
	//
	// The check stays registered, but no release.yaml declares the key: such a role loses that access
	// at the upgrade to DKP 1.78 already, so holding a later release would give nothing back, and the
	// D8UserAuthzLegacyRBACv2CustomRoleFound alert names the role instead.
	legacyRBACv2CustomRolesRequirementKey = "legacyRBACv2CustomRolesCount"

	migrationFAQReference = "see the user-authz module FAQ, section \"How do I migrate custom roles to the new scheme?\""

	// deprecatedRBACv2BindingsRequirementKey is the release requirement key of the release that
	// removes the compatibility aliases of the previous RBACv2 role names (d8:manage:*,
	// d8:use:role:*). That release.yaml sets it to the maximum allowed number of bindings to the
	// deprecated names (0): with the aliases gone such a binding grants nothing, so the release
	// stays Pending until every binding is recreated on the new name.
	deprecatedRBACv2BindingsRequirementKey = "deprecatedRBACv2BindingsCount"
	deprecatedNamesReference               = "see the user-authz module documentation, section \"Deprecated role names\", and the D8UserAuthzDeprecatedRBACv2RoleInUse / D8UserAuthzDeprecatedRBACv2CapabilityInUse alerts"
)

func init() {
	checkLegacyCustomRolesFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		allowed, err := strconv.Atoi(requirementValue)
		if err != nil {
			return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
		}

		raw, exists := getter.Get(hooks.LegacyRBACv2CustomRolesValueKey)
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
			"the cluster has %d custom role(s) that have lost access the role model gave them: %s; their "+
				"aggregationRule selects capabilities by the rbac.deckhouse.io/kind: manage or use label, or "+
				"by the aggregation label of a lineage that no built-in role collects and no custom capability "+
				"carries, or they have rbac.deckhouse.io/use-role without the system or subsystem scope, so "+
				"they get no RoleBindings in the namespaces of the modules; replace them with d8:custom:* "+
				"roles of the new scheme, %s",
			len(names), strings.Join(names, ", "), migrationFAQReference)
	}

	requirements.RegisterCheck(legacyRBACv2CustomRolesRequirementKey, checkLegacyCustomRolesFunc)

	checkDeprecatedBindingsFunc := func(requirementValue string, getter requirements.ValueGetter) (bool, error) {
		allowed, err := strconv.Atoi(requirementValue)
		if err != nil {
			return false, fmt.Errorf("parse requirement value %q: %w", requirementValue, err)
		}
		raw, exists := getter.Get(hooks.DeprecatedRBACv2BindingsValueKey)
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
			"the cluster has %d binding(s) to deprecated RBACv2 role names (d8:manage:*, d8:use:role:*): %s; "+
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
