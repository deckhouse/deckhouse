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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// The monitoring-kubernetes-control-plane module was merged into the control-plane-manager module
// (#20734), and its Grafana dashboards moved along with it. helm-lib builds the
// ClusterObservabilityDashboard name as d8-<Chart.Name>-<path>, so the very same dashboards are now
// rendered as d8-control-plane-manager-* instead of the former
// d8-monitoring-kubernetes-control-plane-*. The JSON is byte-identical, hence the uid inside each is
// the same, and the observability webhook rejects a second ClusterObservabilityDashboard carrying
// an already-present uid. Module deploy order cannot be changed, so the control-plane-manager module
// can start rendering the new resources while the stale monitoring-kubernetes-control-plane ones
// still linger in the cluster. Delete the stale resources in beforeHelm so the new ones apply
// cleanly.
var obsoleteMonitoringKubernetesControlPlaneDashboards = []string{
	"d8-monitoring-kubernetes-control-plane-kubernetes-cluster-control-plane-status",
	"d8-monitoring-kubernetes-control-plane-kubernetes-cluster-kube-etcd3",
	"d8-monitoring-kubernetes-control-plane-kubernetes-cluster-deprecated-resources",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, deleteObsoleteMonitoringKubernetesControlPlaneDashboards)

func deleteObsoleteMonitoringKubernetesControlPlaneDashboards(_ context.Context, input *go_hook.HookInput) error {
	// The ClusterObservabilityDashboard CRD exists only when the observability module is enabled;
	// without it there is nothing to delete (and the resources cannot exist).
	if !set.NewFromValues(input.Values, "global.enabledModules").Has("observability") {
		return nil
	}

	// Delete is idempotent: a missing resource is not an error.
	for _, name := range obsoleteMonitoringKubernetesControlPlaneDashboards {
		input.PatchCollector.Delete(
			"observability.deckhouse.io/v1alpha1",
			"ClusterObservabilityDashboard",
			"", // cluster-scoped
			name,
		)
	}

	return nil
}
