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

// Discovers custom ClusterRoles of the LEGACY experimental RBACv2 scheme ("custom:*" names with the
// rbac.deckhouse.io/kind: manage|use labels or aggregation selectors). The new scheme renames the
// labels that drive aggregation, so such roles stop aggregating permissions in DKP 1.78 and have no
// compatibility aliases (unlike the built-in d8:manage:*/d8:use:role:* roles). The discovered names
// are stored as a release requirement value: the DKP 1.78 release carries the
// legacyRBACv2CustomRolesCount requirement and stays Pending until the roles are migrated to the new
// d8:custom:* scheme (see the user-authz FAQ, "How do I migrate custom roles to the new scheme in
// DKP 1.78?").

package hooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	legacyCustomRolesSnapshot = "legacy_custom_roles"

	// LegacyRBACv2CustomRolesValueKey is the requirements memory-storage key holding the sorted names
	// of the legacy custom roles found in the cluster. The requirement check function
	// (modules/140-user-authz/requirements) reads it when a release carries the
	// legacyRBACv2CustomRolesCount requirement.
	LegacyRBACv2CustomRolesValueKey = "userAuthz:legacyRBACv2CustomRoles"

	// legacyCustomRolePrefix is the name prefix the legacy scheme prescribed for user-created roles
	// and capabilities (the new scheme uses d8:custom:).
	legacyCustomRolePrefix = "custom:"

	// legacyCustomRoleMetric drives the D8UserAuthzLegacyRBACv2CustomRoleFound alert: one time series
	// per legacy custom role found in the cluster, so operators learn about the upcoming DKP 1.78
	// breakage (and release block) before attempting the upgrade.
	legacyCustomRoleMetric = "d8_rbacv2_legacy_custom_role"

	rbacKindLabel = "rbac.deckhouse.io/kind"
)

// legacyRBACKinds are the rbac.deckhouse.io/kind label values of the LEGACY scheme. In the new
// scheme built-in objects use role|capability and custom ones use custom-role|custom-capability, so
// manage|use unambiguously identifies a leftover legacy object.
var legacyRBACKinds = map[string]struct{}{
	"manage": {},
	"use":    {},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("legacy_custom_roles"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       legacyCustomRolesSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyLegacyCustomRoleFilter,
		},
	},
}, discoveryLegacyCustomRolesHandler)

// applyLegacyCustomRoleFilter keeps only the legacy-scheme custom roles, so the snapshot never holds
// the rest of the cluster's ClusterRoles.
func applyLegacyCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	if !isLegacyCustomRole(role) {
		return nil, nil
	}
	return role.Name, nil
}

// isLegacyCustomRole reports whether a ClusterRole is a user-created role/capability of the legacy
// experimental RBACv2 scheme: a "custom:*" name combined with the legacy kind label — on the object
// itself (roles and capabilities both carried it) or inside its aggregation selectors (a role whose
// selectors require the legacy kind stops matching the relabeled built-in capabilities just as well).
// A "custom:*" ClusterRole unrelated to the role model (no legacy markers) is not counted: it does
// not break in DKP 1.78 and must not block the release.
func isLegacyCustomRole(role *rbacv1.ClusterRole) bool {
	if !strings.HasPrefix(role.Name, legacyCustomRolePrefix) {
		return false
	}
	// Built-in objects are never named "custom:*"; the heritage check is defense in depth.
	if role.Labels["heritage"] == "deckhouse" {
		return false
	}

	if _, ok := legacyRBACKinds[role.Labels[rbacKindLabel]]; ok {
		return true
	}

	if role.AggregationRule == nil {
		return false
	}
	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		if _, ok := legacyRBACKinds[selector.MatchLabels[rbacKindLabel]]; ok {
			return true
		}
	}
	return false
}

// discoveryLegacyCustomRolesHandler publishes the sorted legacy role names as a requirement value
// and a per-role alert metric on every synchronization/event, so the DKP 1.78 release unblocks
// itself (and the alert resolves) as soon as the operator migrates or deletes the roles.
func discoveryLegacyCustomRolesHandler(_ context.Context, input *go_hook.HookInput) error {
	names := make([]string, 0)
	for name, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get(legacyCustomRolesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", legacyCustomRolesSnapshot, err)
		}
		names = append(names, name)
	}
	slices.Sort(names)

	requirements.SaveValue(LegacyRBACv2CustomRolesValueKey, names)

	input.MetricsCollector.Expire(legacyCustomRoleMetric)
	for _, name := range names {
		input.MetricsCollector.Set(legacyCustomRoleMetric, 1,
			map[string]string{"name": name},
			metrics.WithGroup(legacyCustomRoleMetric))
	}
	return nil
}
