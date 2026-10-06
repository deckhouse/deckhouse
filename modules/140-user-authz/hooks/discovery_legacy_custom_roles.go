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

// Discovers the custom ClusterRoles that have lost access the role model gave them, whatever their
// names. Such a role has no heritage: deckhouse label and has an aggregationRule, and one of the
// following holds:
//
//   - A selector of it requires rbac.deckhouse.io/kind manage or use. The built-in capabilities no
//     longer carry these values, so the selector matches none of them, and the aggregation
//     controller leaves the role only the rules of the custom capabilities that still carry them.
//   - The role has the rbac.deckhouse.io/use-role label without rbac.deckhouse.io/scope system or
//     subsystem. Deckhouse makes the RoleBindings in the namespaces of the modules only for a role
//     with that scope, so the role has lost them, even with its selectors rewritten.
//   - A selector of it requires the aggregation label of a lineage that no built-in role collects
//     and no custom capability carries, such as rbac.deckhouse.io/aggregate-to-all-as of the old
//     scheme, the label of a subsystem the role model no longer has, or that of a subsystem a module
//     has left. The modules label their capabilities only with the lineages the roles of the model
//     collect, so the selector matches none of the built-in capabilities either. A lineage that a
//     custom capability carries is a custom one, and the selector still gets its rules. So is the
//     lineage of the role's own subsystem, which the custom capabilities of that subsystem carry.
//     Neither exception holds for the retired lineages (retiredLineages). A compatibility alias of
//     an old role (rbac.deckhouse.io/deprecated) is not a built-in role that collects a lineage,
//     because it collects the lineage of an old subsystem only for the bindings to the old name.
//
// Custom roles get no compatibility aliases. The sorted names are stored as a requirement value
// that the legacyRBACv2CustomRolesCount check reads, and each name drives the
// D8UserAuthzLegacyRBACv2CustomRoleFound alert, which describes the repair (see also the user-authz
// FAQ, "How do I migrate custom roles to the new scheme?").
//
// A ClusterRole with rules and the kind label but no aggregationRule is not counted: it is a
// capability, or a role frozen by removing its aggregationRule, and it keeps its rules. A capability
// of the old scheme that still carries an aggregation label is reported by
// D8UserAuthzForeignAggregationLabel (alert_foreign_aggregation_label.go).

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	legacyCustomRolesSnapshot = "legacy_custom_roles"

	// collectedLineagesSnapshot holds the lineages the built-in roles in the cluster collect.
	collectedLineagesSnapshot = "collected_lineages"

	// carriedLineagesSnapshot holds the lineages the custom capabilities in the cluster carry.
	carriedLineagesSnapshot = "carried_lineages"

	// LegacyRBACv2CustomRolesValueKey is the requirements memory-storage key holding the sorted names
	// of the legacy custom roles found in the cluster. The requirement check function
	// (modules/140-user-authz/requirements) reads it when a release carries the
	// legacyRBACv2CustomRolesCount requirement.
	LegacyRBACv2CustomRolesValueKey = "userAuthz:legacyRBACv2CustomRoles"

	// legacyCustomRoleMetric drives the D8UserAuthzLegacyRBACv2CustomRoleFound alert: one time series
	// per legacy custom role found in the cluster.
	legacyCustomRoleMetric = "d8_rbacv2_legacy_custom_role"

	rbacKindLabel      = "rbac.deckhouse.io/kind"
	rbacUseRoleLabel   = "rbac.deckhouse.io/use-role"
	rbacScopeLabel     = "rbac.deckhouse.io/scope"
	rbacSubsystemLabel = "rbac.deckhouse.io/subsystem"
)

// legacyRBACKinds are the rbac.deckhouse.io/kind label values of the experimental role model. In the
// new label scheme built-in objects use role|capability and custom ones use
// custom-role|custom-capability, so a selector that requires manage|use relies on the old labels.
var legacyRBACKinds = map[string]struct{}{
	"manage": {},
	"use":    {},
}

// scopeLineages are the lineages of the system, namespace and project scopes. The role model always
// has them, so a selector by them is not compared with the built-in roles in the cluster.
var scopeLineages = []string{"system", "namespace", "project"}

// retiredLineages are rbac.deckhouse.io/aggregate-to-all-as of the old scheme and the lineages of the
// deckhouse, infrastructure, kubernetes and networking subsystems, which the new subsystem list
// replaces. Such a lineage is not one of a custom subsystem, even when a custom role names its own
// subsystem so or a custom capability carries it, and a selector by it is counted unless a built-in
// role in the cluster other than a compatibility alias still collects it.
var retiredLineages = map[string]struct{}{
	"all":            {},
	"deckhouse":      {},
	"infrastructure": {},
	"kubernetes":     {},
	"networking":     {},
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
		{
			Name:       collectedLineagesSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"heritage": "deckhouse"},
			},
			FilterFunc: applyCollectedLineagesFilter,
		},
		{
			Name:       carriedLineagesSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "heritage",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"deckhouse"},
				}},
			},
			FilterFunc: applyCarriedLineagesFilter,
		},
	},
}, discoveryLegacyCustomRolesHandler)

// legacyCustomRoleCandidate is what the snapshot keeps of a custom role that may have lost access.
type legacyCustomRoleCandidate struct {
	Name string `json:"name"`
	// LegacyLabels is set when a selector of the role requires the manage or use kind, or when the
	// role has use-role without the system or subsystem scope.
	LegacyLabels bool `json:"legacyLabels"`
	// Lineages are the lineages its selectors require, other than the scope lineages and the lineage
	// of its own subsystem. Whether a built-in role collects them or a custom capability carries them
	// is known only from the collectedLineagesSnapshot and the carriedLineagesSnapshot, which a filter
	// does not see.
	Lineages []string `json:"lineages"`
}

// applyLegacyCustomRoleFilter keeps only the custom roles that have lost access or select by a
// lineage a built-in role may not collect, so the snapshot never holds the rest of the cluster's
// ClusterRoles.
func applyLegacyCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	candidate, ok := legacyCustomRoleCandidateOf(role)
	if !ok {
		return nil, nil
	}
	return candidate, nil
}

// legacyCustomRoleCandidateOf returns what the hook needs of a custom role of the role model, and
// whether the role may have lost access. Such a role is not a built-in object (heritage:
// deckhouse), has an aggregationRule, and has the labels of the old scheme or selects by a lineage
// other than the scope lineages and the lineage of its own subsystem. The name does not matter. A
// role frozen by removing its aggregationRule is not counted, although it keeps its labels and its
// rules.
func legacyCustomRoleCandidateOf(role *rbacv1.ClusterRole) (legacyCustomRoleCandidate, bool) {
	if role.Labels["heritage"] == "deckhouse" || role.AggregationRule == nil {
		return legacyCustomRoleCandidate{}, false
	}
	candidate := legacyCustomRoleCandidate{
		Name:         role.Name,
		LegacyLabels: hasLegacyLabels(role),
		Lineages:     lineagesToCheck(role),
	}
	return candidate, candidate.LegacyLabels || len(candidate.Lineages) > 0
}

// hasLegacyLabels reports whether a custom role relies on the labels of the old scheme, that is,
// whether a selector of it requires rbac.deckhouse.io/kind manage or use, or the role has the
// rbac.deckhouse.io/use-role label without rbac.deckhouse.io/scope system or subsystem.
func hasLegacyLabels(role *rbacv1.ClusterRole) bool {
	if role.Labels[rbacUseRoleLabel] != "" && !slices.Contains(projectedScopes, role.Labels[rbacScopeLabel]) {
		return true
	}
	return slices.ContainsFunc(role.AggregationRule.ClusterRoleSelectors, selectorRequiresLegacyKind)
}

// projectedScopes are the rbac.deckhouse.io/scope values of the roles whose use-role Deckhouse
// projects into RoleBindings in the namespaces of the modules.
var projectedScopes = []string{"system", "subsystem"}

// selectorRequiresLegacyKind reports whether a selector relies on the manage or use value of
// rbac.deckhouse.io/kind, in its match labels or in an In expression. Exists and NotIn expressions
// match the labels of the new scheme as well and do not count.
func selectorRequiresLegacyKind(selector metav1.LabelSelector) bool {
	if _, ok := legacyRBACKinds[selector.MatchLabels[rbacKindLabel]]; ok {
		return true
	}
	for _, expression := range selector.MatchExpressions {
		if expression.Key != rbacKindLabel || expression.Operator != metav1.LabelSelectorOpIn {
			continue
		}
		for _, value := range expression.Values {
			if _, ok := legacyRBACKinds[value]; ok {
				return true
			}
		}
	}
	return false
}

// lineagesToCheck returns the sorted lineages the selectors of a custom role require, except the
// scope lineages and the lineage of the role's own subsystem (rbac.deckhouse.io/subsystem), unless
// that one is retired.
func lineagesToCheck(role *rbacv1.ClusterRole) []string {
	ownSubsystem := role.Labels[rbacSubsystemLabel]
	_, ownSubsystemRetired := retiredLineages[ownSubsystem]
	lineages := make([]string, 0)
	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		for _, lineage := range requiredLineages(selector) {
			if slices.Contains(scopeLineages, lineage) || (lineage == ownSubsystem && !ownSubsystemRetired) {
				continue
			}
			lineages = append(lineages, lineage)
		}
	}
	slices.Sort(lineages)
	return slices.Compact(lineages)
}

// requiredLineages returns the lineages whose rbac.deckhouse.io/aggregate-to-<lineage>-as label a
// selector requires, by a key of its match labels or by the key of an In or Exists expression. NotIn
// and DoesNotExist expressions match objects without the label as well and require nothing.
func requiredLineages(selector metav1.LabelSelector) []string {
	lineages := make([]string, 0)
	for key := range selector.MatchLabels {
		if lineage, ok := lineageOfLabel(key); ok {
			lineages = append(lineages, lineage)
		}
	}
	for _, expression := range selector.MatchExpressions {
		if expression.Operator != metav1.LabelSelectorOpIn && expression.Operator != metav1.LabelSelectorOpExists {
			continue
		}
		if lineage, ok := lineageOfLabel(expression.Key); ok {
			lineages = append(lineages, lineage)
		}
	}
	return lineages
}

// lineageOfLabel returns the lineage of an rbac.deckhouse.io/aggregate-to-<lineage>-as label key,
// and whether the key is one.
func lineageOfLabel(key string) (string, bool) {
	lineage, ok := strings.CutPrefix(key, aggregationLabelPrefix)
	if !ok {
		return "", false
	}
	lineage, ok = strings.CutSuffix(lineage, aggregationLabelSuffix)
	return lineage, ok && lineage != ""
}

// applyCollectedLineagesFilter keeps the sorted lineages that the selectors of a built-in role
// require. These are the lineages the role model collects in this cluster, those of the subsystems
// of the platform and of the subsystems a module ships roles of its own for. A compatibility alias of
// an old role (rbac.deckhouse.io/deprecated) is left out. The aliases of the old subsystem roles keep
// selecting the old lineages for the bindings to the old names after the built-in modules have moved
// to the new subsystems, so a selector by such a lineage gets none of the rules of these modules.
func applyCollectedLineagesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	if role.AggregationRule == nil || role.Labels[deprecatedLabel] == "true" {
		return nil, nil
	}
	lineages := make([]string, 0)
	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		lineages = append(lineages, requiredLineages(selector)...)
	}
	if len(lineages) == 0 {
		return nil, nil
	}
	slices.Sort(lineages)
	return slices.Compact(lineages), nil
}

// applyCarriedLineagesFilter keeps the sorted lineages whose aggregation labels a custom capability
// carries: a ClusterRole without heritage: deckhouse and without an aggregationRule. A selector by
// such a lineage gets the rules of the capability, so a custom role that selects by it has lost
// nothing. A custom role is not a carrier, or a role would vouch for the lineage it selects by.
func applyCarriedLineagesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	if role.Labels["heritage"] == "deckhouse" || role.AggregationRule != nil {
		return nil, nil
	}
	lineages := make([]string, 0)
	for key := range role.Labels {
		if lineage, ok := lineageOfLabel(key); ok {
			lineages = append(lineages, lineage)
		}
	}
	if len(lineages) == 0 {
		return nil, nil
	}
	slices.Sort(lineages)
	return lineages, nil
}

// discoveryLegacyCustomRolesHandler publishes the sorted legacy role names as a requirement value
// and a per-role alert metric on every synchronization or event, so the requirement passes and the
// alert resolves as soon as the roles are frozen, migrated or deleted.
func discoveryLegacyCustomRolesHandler(_ context.Context, input *go_hook.HookInput) error {
	collected := make(map[string]struct{})
	for _, lineage := range scopeLineages {
		collected[lineage] = struct{}{}
	}
	for lineages, err := range sdkobjectpatch.SnapshotIter[[]string](input.Snapshots.Get(collectedLineagesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", collectedLineagesSnapshot, err)
		}
		for _, lineage := range lineages {
			collected[lineage] = struct{}{}
		}
	}
	carried := make(map[string]struct{})
	for lineages, err := range sdkobjectpatch.SnapshotIter[[]string](input.Snapshots.Get(carriedLineagesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", carriedLineagesSnapshot, err)
		}
		for _, lineage := range lineages {
			carried[lineage] = struct{}{}
		}
	}
	// isLost reports whether a selector by the lineage gets no rules the role model or the custom
	// capabilities give: no built-in role collects it, and it is retired or no custom capability
	// carries it.
	isLost := func(lineage string) bool {
		if _, ok := collected[lineage]; ok {
			return false
		}
		if _, ok := retiredLineages[lineage]; ok {
			return true
		}
		_, ok := carried[lineage]
		return !ok
	}

	names := make([]string, 0)
	for candidate, err := range sdkobjectpatch.SnapshotIter[legacyCustomRoleCandidate](input.Snapshots.Get(legacyCustomRolesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", legacyCustomRolesSnapshot, err)
		}
		if candidate.LegacyLabels || slices.ContainsFunc(candidate.Lineages, isLost) {
			names = append(names, candidate.Name)
		}
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
