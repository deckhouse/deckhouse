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

package resources

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
)

const (
	autotuneMetricName  = "d8_control_plane_manager_resources_autotune_insufficient_capacity"
	autotuneMetricGroup = "D8ControlPlaneResourcesAutotuneInsufficientCapacity"

	obsoleteGlobalResourcesRequestsMetricName  = "d8_obsolete_global_control_plane_resources_requests"
	obsoleteGlobalResourcesRequestsMetricGroup = "D8ObsoleteGlobalControlPlaneResourcesRequests"

	// Each group expires independently, hence a separate group per hook.
	autotuneDegradedMetricName      = "d8_control_plane_manager_resources_autotune_degraded"
	autotuneDegradedMetricGroup     = "D8ControlPlaneResourcesAutotuneDegraded"
	autotuneSyncDegradedMetricGroup = "D8ControlPlaneResourcesAutotuneSyncDegraded"

	autotuneIncompleteMetricName  = "d8_control_plane_manager_resources_autotune_state_incomplete"
	autotuneIncompleteMetricGroup = "D8ControlPlaneResourcesAutotuneStateIncomplete"
)

const (
	degradedReasonBadNodes      = "bad_nodes"
	degradedReasonBadState      = "bad_state"
	degradedReasonBadOverride   = "bad_override"
	degradedReasonNodesTooSmall = "nodes_too_small"
	degradedReasonListPods      = "list_pods"
	degradedReasonReadThrough   = "read_through"
)

func setAutotuneDegraded(input *go_hook.HookInput, group, reason string) {
	input.MetricsCollector.Set(
		autotuneDegradedMetricName,
		1,
		map[string]string{"reason": reason},
		metrics.WithGroup(group),
	)
}
