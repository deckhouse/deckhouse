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

package hooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

// The new subsystem list merged deckhouse, infrastructure and kubernetes into cluster and renamed
// networking to network. No d8:system:* or d8:subsystem:* role collects the aggregation labels of
// these four subsystems any more, so the rules of a ClusterRole that still carries one reach none of
// them. Only the compatibility aliases of the old roles still collect such a label, for one release.
// The template test keeps the modules of this repository off these labels, but a module released on
// its own and a custom capability may still carry one. This hook exports every such ClusterRole,
// whoever made it, and the alert on the metric names the lineage to label it with instead.
//
// The previous scheme also labelled its namespace-level use capabilities with
// rbac.deckhouse.io/kind: use and the kubernetes lineage, and a module with a subsystem of its own
// labelled its use roles, such as d8:use:role:<level>:virtualization, with rbac.deckhouse.io/kind:
// use and the all lineage. The d8:namespace:* and d8:project:* roles collect neither, only the
// compatibility aliases d8:use:role:* do, so a RoleBinding moved to a new role loses their rules. A
// use capability with the lineage of the system scope, of a subsystem or of deckhouse, infrastructure
// or networking reaches no role at all: the d8:system:* and d8:subsystem:* roles and the d8:manage:*
// aliases leave rbac.deckhouse.io/kind: use out. The hook exports every such ClusterRole as a series
// of its own, with the lineage it carries, and the alert on it says that the module or the capability
// is still on the previous scheme.
const (
	retiredAggregationLabelMetric   = "d8_user_authz_retired_aggregation_label"
	legacyUseCapabilityMetric       = "d8_user_authz_legacy_use_capability"
	retiredAggregationLabelSnapshot = "retired_aggregation_label_clusterroles"

	// legacyUseLineage is the lineage the previous scheme put on its use capabilities.
	legacyUseLineage = "kubernetes"
	// legacyUseRoleLineage is the lineage of the use roles of a module with a subsystem of its own in
	// the previous scheme, which d8:use:role:<level> collected.
	legacyUseRoleLineage = "all"
)

// uncollectedUseLineages are the lineages under which no role collects a ClusterRole with
// rbac.deckhouse.io/kind: use: the system lineage and the lineages of the subsystems, whose roles leave
// such a ClusterRole out, and deckhouse, infrastructure and networking, whose aliases leave it out as
// well. The lineage of the subsystem of a module, such as virtualization, is not one of them: the use
// roles of the module collect it.
var uncollectedUseLineages = map[string]struct{}{
	"system":           {},
	"iam":              {},
	"security":         {},
	"cluster":          {},
	"delivery":         {},
	"network":          {},
	"storage":          {},
	"observability":    {},
	"managed-services": {},
	"deckhouse":        {},
	"infrastructure":   {},
	"networking":       {},
}

// retiredLineageReplacements maps the lineage of a subsystem the new list merged or renamed to the
// subsystem that took over most of its modules.
var retiredLineageReplacements = map[string]string{
	"deckhouse":      "cluster",
	"infrastructure": "cluster",
	"kubernetes":     "cluster",
	"networking":     "network",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("retired-aggregation-label"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       retiredAggregationLabelSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: filterRetiredAggregationLabelClusterRole,
		},
	},
}, handleRetiredAggregationLabels)

// retiredLineageRole is the projection of a ClusterRole that carries a retired aggregation label, or
// of a use capability or use role of the previous scheme, which has its lineages in UseLineages. The
// FilterFunc returns nil for every other role, so the snapshot holds these roles only.
type retiredLineageRole struct {
	Name        string       `json:"name"`
	Module      string       `json:"module"`
	Lineages    []string     `json:"lineages"`
	UseLineages []useLineage `json:"useLineages"`
}

// useLineage is a lineage of a use capability or use role of the previous scheme and its level.
type useLineage struct {
	Lineage string `json:"lineage"`
	Level   string `json:"level"`
}

func filterRetiredAggregationLabelClusterRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()
	// The old manage roles never took the use capabilities of the old scheme, so their retired labels
	// are not what the retired label alert is about. The new namespace and project roles do not take
	// them either, and that is what the use capability alert is about.
	if labels[rbacKindLabel] == "use" {
		for _, lineage := range []string{legacyUseLineage, legacyUseRoleLineage} {
			if level, ok := labels[aggregationLabelPrefix+lineage+aggregationLabelSuffix]; ok {
				return retiredLineageRole{
					Name:        obj.GetName(),
					Module:      labels["module"],
					UseLineages: []useLineage{{Lineage: lineage, Level: level}},
				}, nil
			}
		}
		// Under the lineages of uncollectedUseLineages no role collects the ClusterRole, so only a
		// binding to the ClusterRole itself gives its rules. Each of them it carries is reported.
		uses := make([]useLineage, 0)
		for key, level := range labels {
			lineage, ok := lineageOfLabel(key)
			if !ok {
				continue
			}
			if _, ok := uncollectedUseLineages[lineage]; ok {
				uses = append(uses, useLineage{Lineage: lineage, Level: level})
			}
		}
		if len(uses) == 0 {
			return nil, nil
		}
		slices.SortFunc(uses, func(a, b useLineage) int { return strings.Compare(a.Lineage, b.Lineage) })
		return retiredLineageRole{Name: obj.GetName(), Module: labels["module"], UseLineages: uses}, nil
	}
	lineages := make([]string, 0)
	for key := range labels {
		if !strings.HasPrefix(key, aggregationLabelPrefix) || !strings.HasSuffix(key, aggregationLabelSuffix) {
			continue
		}
		lineage := strings.TrimSuffix(strings.TrimPrefix(key, aggregationLabelPrefix), aggregationLabelSuffix)
		if _, ok := retiredLineageReplacements[lineage]; ok {
			lineages = append(lineages, lineage)
		}
	}
	if len(lineages) == 0 {
		return nil, nil
	}
	slices.Sort(lineages)
	return retiredLineageRole{Name: obj.GetName(), Module: labels["module"], Lineages: lineages}, nil
}

func handleRetiredAggregationLabels(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(retiredAggregationLabelMetric)
	input.MetricsCollector.Expire(legacyUseCapabilityMetric)

	for role, err := range sdkobjectpatch.SnapshotIter[retiredLineageRole](input.Snapshots.Get(retiredAggregationLabelSnapshot)) {
		if err != nil {
			return fmt.Errorf("iterate over the %q snapshot: %w", retiredAggregationLabelSnapshot, err)
		}
		for _, use := range role.UseLineages {
			input.MetricsCollector.Set(legacyUseCapabilityMetric, 1, map[string]string{
				"name":    role.Name,
				"module":  role.Module,
				"lineage": use.Lineage,
				"level":   use.Level,
			}, metrics.WithGroup(legacyUseCapabilityMetric))
		}
		for _, lineage := range role.Lineages {
			input.MetricsCollector.Set(retiredAggregationLabelMetric, 1, map[string]string{
				"name":        role.Name,
				"module":      role.Module,
				"lineage":     lineage,
				"replaced_by": retiredLineageReplacements[lineage],
			}, metrics.WithGroup(retiredAggregationLabelMetric))
		}
	}
	return nil
}
