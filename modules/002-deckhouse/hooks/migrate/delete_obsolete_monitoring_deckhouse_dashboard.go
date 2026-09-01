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

package migrate

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// The monitoring-deckhouse module was merged into the deckhouse module (#20727), and its
// Grafana dashboard main/deckhouse.json moved along with it. helm-lib builds the
// ClusterObservabilityDashboard name as d8-<Chart.Name>-<path>, so the very same dashboard is
// now rendered as d8-deckhouse-main-deckhouse instead of the former
// d8-monitoring-deckhouse-main-deckhouse. The JSON is byte-identical, hence the uid inside it is
// the same, and the observability webhook rejects a second ClusterObservabilityDashboard carrying
// an already-present uid. Module deploy order cannot be changed, so the deckhouse module can start
// rendering the new resource while the stale monitoring-deckhouse one still lingers in the cluster.
// Delete the stale resource in beforeHelm so the new one applies cleanly.
const obsoleteMonitoringDeckhouseDashboard = "d8-monitoring-deckhouse-main-deckhouse"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, deleteObsoleteMonitoringDeckhouseDashboard)

func deleteObsoleteMonitoringDeckhouseDashboard(_ context.Context, input *go_hook.HookInput) error {
	// The ClusterObservabilityDashboard CRD exists only when the observability module is enabled;
	// without it there is nothing to delete (and the resource cannot exist).
	if !set.NewFromValues(input.Values, "global.enabledModules").Has("observability") {
		return nil
	}

	// Delete is idempotent: a missing resource is not an error.
	input.PatchCollector.Delete(
		"observability.deckhouse.io/v1alpha1",
		"ClusterObservabilityDashboard",
		"", // cluster-scoped
		obsoleteMonitoringDeckhouseDashboard,
	)

	return nil
}
