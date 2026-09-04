/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	customClusterRoleSnapshots    = "custom_cluster_roles"
	aggregatedCustomRoleSnapshots = "aggregated_custom_cluster_roles"

	// accessLevelKey is both the annotation that marks a custom ClusterRole and the label
	// that the aggregated ClusterRoles (user-authz:<level>:custom) select it by. The
	// annotation is the public contract; the label is maintained by this hook. Nothing else
	// is derived from custom roles anymore: the aggregation happens in Kubernetes and the
	// bindings are reconciled by user-authz-controller.
	accessLevelKey = "user-authz.deckhouse.io/access-level"

	accessLevelUser           = "User"
	accessLevelPrivilegedUser = "PrivilegedUser"
	accessLevelEditor         = "Editor"
	accessLevelAdmin          = "Admin"
	accessLevelClusterEditor  = "ClusterEditor"
	accessLevelClusterAdmin   = "ClusterAdmin"
)

type customClusterRole struct {
	Name string
	// Role is the access level from the annotation; empty when the annotation is absent or
	// invalid (such a role is in the snapshot only to have a stale label removed).
	Role string
	// Label is the current value of the access-level label on the object.
	Label string
}

func applyCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ccr := &customClusterRole{
		Name:  obj.GetName(),
		Label: obj.GetLabels()[accessLevelKey],
	}

	role := obj.GetAnnotations()[accessLevelKey]
	switch role {
	case accessLevelUser, accessLevelPrivilegedUser, accessLevelEditor, accessLevelAdmin, accessLevelClusterEditor, accessLevelClusterAdmin:
		ccr.Role = role
	default:
		if ccr.Label == "" {
			return nil, nil
		}
	}

	return ccr, nil
}

// accessLevelsInOrder lists the levels from the narrowest to the widest: the aggregated role of a
// level selects the custom roles of that level and of every level before it.
var accessLevelsInOrder = []string{accessLevelUser, accessLevelPrivilegedUser, accessLevelEditor, accessLevelAdmin, accessLevelClusterEditor, accessLevelClusterAdmin}

// aggregatedCustomRoleNames maps the aggregated ClusterRoles rendered by the chart to their level.
var aggregatedCustomRoleNames = map[string]string{
	"user-authz:user:custom":            accessLevelUser,
	"user-authz:privileged-user:custom": accessLevelPrivilegedUser,
	"user-authz:editor:custom":          accessLevelEditor,
	"user-authz:admin:custom":           accessLevelAdmin,
	"user-authz:cluster-editor:custom":  accessLevelClusterEditor,
	"user-authz:cluster-admin:custom":   accessLevelClusterAdmin,
}

// aggregatedCustomRole is what the hook keeps of an aggregated ClusterRole: how many rules
// kube-controller-manager has collected into it.
type aggregatedCustomRole struct {
	Level string
	Rules int
}

func applyAggregatedCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	level, ok := aggregatedCustomRoleNames[obj.GetName()]
	if !ok {
		return nil, nil
	}
	rules, _, err := unstructured.NestedSlice(obj.Object, "rules")
	if err != nil {
		return nil, fmt.Errorf("read rules of %s: %w", obj.GetName(), err)
	}
	return aggregatedCustomRole{Level: level, Rules: len(rules)}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("custom_rbac_roles"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       customClusterRoleSnapshots,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyCustomRoleFilter,
		},
		{
			Name:         aggregatedCustomRoleSnapshots,
			ApiVersion:   "rbac.authorization.k8s.io/v1",
			Kind:         "ClusterRole",
			NameSelector: &types.NameSelector{MatchNames: slices.Sorted(maps.Keys(aggregatedCustomRoleNames))},
			FilterFunc:   applyAggregatedCustomRoleFilter,
		},
	},
}, customClusterRolesHandler)

const (
	// customClusterRolesMetric counts the custom ClusterRoles per access level: the size of the
	// aggregated roles user-authz:<level>:custom and what the module release used to grow with.
	customClusterRolesMetric = "d8_user_authz_custom_cluster_roles"
	// customAggregationMissingMetric is 1 for a level whose aggregated ClusterRole has no rules
	// although custom roles it must aggregate exist: the aggregation controller is not running, the
	// role is missing, or the union does not fit one object. Users of that level then lack every
	// custom permission.
	customAggregationMissingMetric = "d8_user_authz_custom_aggregation_missing"
)

func customClusterRolesHandler(_ context.Context, input *go_hook.HookInput) error {
	roles, err := snapshotsToCustomClusterRoles(input.Snapshots.Get(customClusterRoleSnapshots))
	if err != nil {
		return fmt.Errorf("failed to convert custom cluster roles snapshots: %w", err)
	}
	aggregated, err := snapshotsToAggregatedCustomRoles(input.Snapshots.Get(aggregatedCustomRoleSnapshots))
	if err != nil {
		return fmt.Errorf("failed to convert aggregated custom roles snapshots: %w", err)
	}

	syncAccessLevelLabels(input, roles)
	exportCustomClusterRolesMetrics(input, roles, aggregated)

	return nil
}

// exportCustomClusterRolesMetrics publishes the number of custom ClusterRoles per access level and
// flags the levels whose aggregated role stayed empty although it has roles to aggregate.
func exportCustomClusterRolesMetrics(input *go_hook.HookInput, roles []customClusterRole, aggregated map[string]int) {
	input.MetricsCollector.Expire(customClusterRolesMetric)
	input.MetricsCollector.Expire(customAggregationMissingMetric)

	perLevel := make(map[string]int)
	for _, role := range roles {
		if role.Role != "" {
			perLevel[role.Role]++
		}
	}
	for level, count := range perLevel {
		input.MetricsCollector.Set(customClusterRolesMetric, float64(count), map[string]string{"level": level}, metrics.WithGroup(customClusterRolesMetric))
	}

	expected := 0
	for _, level := range accessLevelsInOrder {
		expected += perLevel[level]
		missing := 0.0
		if expected > 0 && aggregated[level] == 0 {
			missing = 1
		}
		input.MetricsCollector.Set(customAggregationMissingMetric, missing, map[string]string{"level": level}, metrics.WithGroup(customAggregationMissingMetric))
	}
}

func snapshotsToAggregatedCustomRoles(snapshots []pkg.Snapshot) (map[string]int, error) {
	rules := make(map[string]int, len(aggregatedCustomRoleNames))
	for role, err := range sdkobjectpatch.SnapshotIter[aggregatedCustomRole](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over '%s' snapshot: %w", aggregatedCustomRoleSnapshots, err)
		}
		rules[role.Level] = role.Rules
	}
	return rules, nil
}

// syncAccessLevelLabels makes the access-level label of every custom ClusterRole equal to its
// annotation, and removes the label from roles that lost the annotation. The label is what the
// aggregated ClusterRoles select by, so a wrong label would grant or revoke rights.
func syncAccessLevelLabels(input *go_hook.HookInput, roles []customClusterRole) {
	for _, role := range roles {
		if role.Label == role.Role {
			continue
		}

		var label any
		if role.Role != "" {
			label = role.Role
		}

		patch := map[string]any{
			"metadata": map[string]any{
				"labels": map[string]any{
					accessLevelKey: label,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "rbac.authorization.k8s.io/v1", "ClusterRole", "", role.Name)
	}
}

func snapshotsToCustomClusterRoles(snapshots []pkg.Snapshot) ([]customClusterRole, error) {
	roles := make([]customClusterRole, 0, len(snapshots))

	for role, err := range sdkobjectpatch.SnapshotIter[customClusterRole](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over '%s' snapshot: %w", customClusterRoleSnapshots, err)
		}

		roles = append(roles, role)
	}

	return roles, nil
}
