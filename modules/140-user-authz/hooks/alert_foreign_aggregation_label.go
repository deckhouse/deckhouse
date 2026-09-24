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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// A ClusterRole labelled rbac.deckhouse.io/aggregate-to-<lineage>-as is merged into the platform
// role of that lineage by the aggregation controller, which reads nothing but the label. The
// cluster_roles webhook now admits the label only on a custom capability, but a webhook sees
// writes: a role that already carried the label before the check existed keeps aggregating after
// the upgrade, and its next UPDATE is refused without warning. This hook is what surfaces both: it
// exports every such role as a metric, and the alert on it says what to do before the role is
// touched. Roles the platform itself renders (heritage: deckhouse) are the platform roles and the
// built-in capabilities; they are held to the contract by the template test instead.
const (
	foreignAggregationLabelMetric = "d8_user_authz_foreign_aggregation_label"
	aggregationLabelPrefix        = "rbac.deckhouse.io/aggregate-to-"
	aggregationLabelSuffix        = "-as"
	customCapabilityKind          = "custom-capability"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authz/foreign-aggregation-label",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "foreign_aggregating_clusterroles",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: filterForeignAggregatingClusterRole,
		},
	},
}, handleForeignAggregationLabels)

// foreignAggregatingRole is the projection of an offending ClusterRole. The FilterFunc returns nil
// for every other role, so the snapshot holds the offenders only.
type foreignAggregatingRole struct {
	Name     string   `json:"name"`
	Lineages []string `json:"lineages"`
	Kind     string   `json:"kind"`
}

func filterForeignAggregatingClusterRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()
	if labels["heritage"] == "deckhouse" || labels[rbacKindLabel] == customCapabilityKind {
		return nil, nil
	}
	var lineages []string
	for key := range labels {
		if strings.HasPrefix(key, aggregationLabelPrefix) && strings.HasSuffix(key, aggregationLabelSuffix) {
			lineages = append(lineages, strings.TrimSuffix(strings.TrimPrefix(key, aggregationLabelPrefix), aggregationLabelSuffix))
		}
	}
	if len(lineages) == 0 {
		return nil, nil
	}
	return foreignAggregatingRole{Name: obj.GetName(), Lineages: lineages, Kind: labels[rbacKindLabel]}, nil
}

func handleForeignAggregationLabels(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(foreignAggregationLabelMetric)

	for role, err := range sdkobjectpatch.SnapshotIter[foreignAggregatingRole](input.Snapshots.Get("foreign_aggregating_clusterroles")) {
		if err != nil {
			return fmt.Errorf("iterate over the 'foreign_aggregating_clusterroles' snapshot: %w", err)
		}
		for _, lineage := range role.Lineages {
			input.MetricsCollector.Set(foreignAggregationLabelMetric, 1, map[string]string{
				"name":    role.Name,
				"lineage": lineage,
				"kind":    role.Kind,
			}, metrics.WithGroup(foreignAggregationLabelMetric))
		}
	}
	return nil
}
