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

// Discovers the custom ClusterRoles that lose access in the new role model, whatever their names.
// Such a role has no heritage: deckhouse label, and its aggregationRule has a selector that
// requires one of the following:
//
//   - rbac.deckhouse.io/kind manage or use of the experimental role model. In the new label scheme
//     the built-in capabilities do not carry these values, so such a selector matches none of them,
//     and the aggregation controller leaves the role without their rules.
//   - The aggregation label of a lineage that no role of the new role model collects, such as
//     rbac.deckhouse.io/aggregate-to-all-as, the label of a subsystem the new subsystem list
//     replaces, or that of a subsystem a module leaves for one of the new subsystems, such as
//     rbac.deckhouse.io/aggregate-to-virtualization-as. In the new role model the modules label
//     their capabilities only with the lineages its roles collect, so such a selector matches none
//     of the built-in capabilities either. A lineage that a custom capability carries is a custom
//     one, and the selector keeps getting its rules after the update. So is the lineage of the role's
//     own subsystem, which the custom capabilities of that subsystem carry. Neither exception holds
//     for the retired lineages (retiredLineages).
//
// A custom role with an aggregationRule is also counted, whatever its selectors, when it carries the
// rbac.deckhouse.io/kind: manage and rbac.deckhouse.io/use-role labels itself. For every
// ClusterRoleBinding to such a role Deckhouse creates RoleBindings to d8:use:role:<level> in the
// namespaces of the modules whose capabilities the role aggregates (handle_manage_bindings.go). The
// new role model creates them only for a role of the system or subsystem scope, which a custom role
// of the old scheme does not have, so the update deletes them, and only explicit copies made before
// it keep that access. The role keeps its own rules when its selectors rely on neither of the above,
// so the metric tells it apart by its reason label, and the alert asks only for the copies.
//
// The check runs before the update, so it compares the lineages with those of the new role model
// rather than with the roles in the cluster. A module that keeps a subsystem of its own after the
// update ships its roles only then, and a selector by the lineage of that subsystem is counted too.
//
// Custom roles get no compatibility aliases. The sorted names are stored as a requirement value that
// the legacyRBACv2CustomRolesCount check reads, and each name drives the
// D8UserAuthzLegacyRBACv2CustomRoleFound alert.
//
// A ClusterRole with rules and the kind label but no aggregationRule is not counted: it is a
// capability, or a role frozen as the user-authz FAQ describes ("How do I migrate custom roles to
// the new scheme?"), and it keeps its rules whatever the built-in capabilities carry. Deckhouse
// creates no RoleBindings in the namespaces of the modules for such a role either. What such an
// object adds to the namespace roles through the old labels is counted by
// discovery_legacy_custom_capabilities.go.

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

	// carriedLineagesSnapshot holds the lineages the custom capabilities in the cluster carry.
	carriedLineagesSnapshot = "carried_lineages"

	// LegacyRBACv2CustomRolesValueKey is the requirements memory-storage key holding the sorted names
	// of the legacy custom roles found in the cluster. The requirement check function
	// (modules/140-user-authz/requirements) reads it when a release carries the
	// legacyRBACv2CustomRolesCount requirement.
	LegacyRBACv2CustomRolesValueKey = "userAuthz:legacyRBACv2CustomRoles"

	// legacyCustomRoleMetric drives the D8UserAuthzLegacyRBACv2CustomRoleFound alert: one time series
	// per legacy custom role found in the cluster, labelled with its name and the reason it is counted
	// for.
	legacyCustomRoleMetric = "d8_rbacv2_legacy_custom_role"

	// Values of the reason label of legacyCustomRoleMetric. A role counted for its selectors has to be
	// frozen. A role counted only for its kind and use-role labels loses nothing but the RoleBindings
	// in the namespaces of the modules, and needs only explicit copies of them.
	legacyCustomRoleSelectorsReason = "selectors"
	legacyCustomRoleUseRoleReason   = "use-role"

	rbacKindLabel      = "rbac.deckhouse.io/kind"
	rbacSubsystemLabel = "rbac.deckhouse.io/subsystem"
	rbacUseRoleLabel   = "rbac.deckhouse.io/use-role"

	aggregationLabelPrefix = "rbac.deckhouse.io/aggregate-to-"
	aggregationLabelSuffix = "-as"
)

// legacyRBACKinds are the rbac.deckhouse.io/kind label values of the experimental role model. In the
// new label scheme built-in objects use role|capability and custom ones use
// custom-role|custom-capability, so a selector that requires manage|use relies on the old labels.
var legacyRBACKinds = map[string]struct{}{
	"manage": {},
	"use":    {},
}

// newModelLineages are the lineages that the roles of the new role model collect, those of the
// system, namespace and project scopes and those of the subsystems of the new subsystem list.
var newModelLineages = map[string]struct{}{
	"system":           {},
	"namespace":        {},
	"project":          {},
	"iam":              {},
	"security":         {},
	"cluster":          {},
	"delivery":         {},
	"network":          {},
	"storage":          {},
	"observability":    {},
	"managed-services": {},
}

// retiredLineages are rbac.deckhouse.io/aggregate-to-all-as of the experimental role model and the
// lineages of the deckhouse, infrastructure, kubernetes and networking subsystems, which the new
// subsystem list replaces. Such a lineage is not one of a custom subsystem, even when a custom role
// names its own subsystem so or a custom capability carries it, and a selector by it is counted.
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

// legacyCustomRoleCandidate is what the snapshot keeps of a custom role that may lose access.
type legacyCustomRoleCandidate struct {
	Name string `json:"name"`
	// LegacyKind is set when a selector of the role requires the manage or use kind.
	LegacyKind bool `json:"legacyKind"`
	// UseRole is set when the role carries the manage kind and the use-role label, so Deckhouse
	// creates RoleBindings for its subjects in the namespaces of the modules.
	UseRole bool `json:"useRole"`
	// Lineages are the lineages its selectors require that no role of the new role model collects,
	// other than the lineage of its own subsystem. Whether a custom capability carries them is known
	// only from the carriedLineagesSnapshot, which a filter does not see.
	Lineages []string `json:"lineages"`
}

// applyLegacyCustomRoleFilter keeps only the custom roles that lose access or select by a lineage
// the new role model does not collect, so the snapshot never holds the rest of the cluster's
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

// legacyCustomRoleCandidateOf returns what the hook needs of a custom role, and whether the role may
// lose access in the new role model. Such a role is not a built-in object (heritage: deckhouse), has
// an aggregationRule, and a selector of it requires rbac.deckhouse.io/kind manage or use, or the
// aggregation label of a lineage that no role of the new role model collects, or the role carries
// the manage kind and the use-role label. The name does not matter, and the other labels of the role
// do not either, except its own subsystem. A role frozen by removing its aggregationRule keeps its
// labels and its rules.
func legacyCustomRoleCandidateOf(role *rbacv1.ClusterRole) (legacyCustomRoleCandidate, bool) {
	if role.Labels["heritage"] == "deckhouse" || role.AggregationRule == nil {
		return legacyCustomRoleCandidate{}, false
	}
	candidate := legacyCustomRoleCandidate{
		Name:       role.Name,
		LegacyKind: slices.ContainsFunc(role.AggregationRule.ClusterRoleSelectors, selectorRequiresLegacyKind),
		UseRole:    role.Labels[rbacKindLabel] == "manage" && role.Labels[rbacUseRoleLabel] != "",
		Lineages:   uncollectedLineages(role),
	}
	return candidate, candidate.LegacyKind || candidate.UseRole || len(candidate.Lineages) > 0
}

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

// uncollectedLineages returns the sorted lineages whose aggregation labels the selectors of a custom
// role require and that no role of the new role model collects. The lineage of the role's own
// subsystem (rbac.deckhouse.io/subsystem) is left out, unless that subsystem has the name of a
// retired lineage.
func uncollectedLineages(role *rbacv1.ClusterRole) []string {
	ownSubsystem := role.Labels[rbacSubsystemLabel]
	_, ownSubsystemRetired := retiredLineages[ownSubsystem]
	lineages := make([]string, 0)
	for _, selector := range role.AggregationRule.ClusterRoleSelectors {
		for _, lineage := range requiredLineages(selector) {
			if _, collected := newModelLineages[lineage]; collected {
				continue
			}
			if lineage == ownSubsystem && !ownSubsystemRetired {
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

// applyCarriedLineagesFilter keeps the sorted lineages whose aggregation labels a custom capability
// carries: a ClusterRole without heritage: deckhouse and without an aggregationRule. A selector by
// such a lineage gets the rules of the capability before and after the update, so a custom role that
// selects by it loses nothing. A custom role is not a carrier, or a role would vouch for the lineage
// it selects by.
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

// legacyCustomRole is a counted custom role and the reason label of its metric.
type legacyCustomRole struct {
	name   string
	reason string
}

// discoveryLegacyCustomRolesHandler publishes the sorted legacy role names as a requirement value
// and a per-role alert metric on every synchronization or event, so the requirement passes and the
// alert resolves as soon as the roles are frozen, migrated or deleted.
func discoveryLegacyCustomRolesHandler(_ context.Context, input *go_hook.HookInput) error {
	carried := make(map[string]struct{})
	for lineages, err := range sdkobjectpatch.SnapshotIter[[]string](input.Snapshots.Get(carriedLineagesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", carriedLineagesSnapshot, err)
		}
		for _, lineage := range lineages {
			carried[lineage] = struct{}{}
		}
	}
	// isLost reports whether a selector by a lineage the new role model does not collect gets no
	// rules after the update: the lineage is retired, or no custom capability carries it.
	isLost := func(lineage string) bool {
		if _, ok := retiredLineages[lineage]; ok {
			return true
		}
		_, ok := carried[lineage]
		return !ok
	}

	roles := make([]legacyCustomRole, 0)
	for candidate, err := range sdkobjectpatch.SnapshotIter[legacyCustomRoleCandidate](input.Snapshots.Get(legacyCustomRolesSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshot: %w", legacyCustomRolesSnapshot, err)
		}
		// A role that loses the rules of its selectors is frozen, and the freeze copies its
		// RoleBindings as well, so the selectors decide the reason of a role counted for both.
		switch {
		case candidate.LegacyKind || slices.ContainsFunc(candidate.Lineages, isLost):
			roles = append(roles, legacyCustomRole{name: candidate.Name, reason: legacyCustomRoleSelectorsReason})
		case candidate.UseRole:
			roles = append(roles, legacyCustomRole{name: candidate.Name, reason: legacyCustomRoleUseRoleReason})
		}
	}
	slices.SortFunc(roles, func(a, b legacyCustomRole) int { return strings.Compare(a.name, b.name) })

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.name)
	}
	requirements.SaveValue(LegacyRBACv2CustomRolesValueKey, names)

	input.MetricsCollector.Expire(legacyCustomRoleMetric)
	for _, role := range roles {
		input.MetricsCollector.Set(legacyCustomRoleMetric, 1,
			map[string]string{"name": role.name, "reason": role.reason},
			metrics.WithGroup(legacyCustomRoleMetric))
	}
	return nil
}
