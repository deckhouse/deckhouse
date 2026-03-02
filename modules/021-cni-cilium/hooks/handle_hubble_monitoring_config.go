/*
Copyright 2025 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/021-cni-cilium/hooks/internal"
)

const (
	hubbleMonitoringConfigSnapshotName = "hubble-monitoring-config-snapshot"
	hubbleMonitoringConfigName         = "hubble-monitoring-config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       hubbleMonitoringConfigSnapshotName,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "HubbleMonitoringConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{hubbleMonitoringConfigName},
			},
			FilterFunc: filterHubbleMonitoringConfig,
		},
	},
}, handleHubbleMonitoringConfig)

func filterHubbleMonitoringConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var hmc internal.HubbleMonitoringConfig
	if err := sdk.FromUnstructured(obj, &hmc); err != nil {
		return nil, fmt.Errorf("cannot convert unstructured to HubbleMonitoringConfig: %w", err)
	}
	return hmc.Spec, nil
}

func handleHubbleMonitoringConfig(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get(hubbleMonitoringConfigSnapshotName)

	// If the HubbleMonitoringConfig CR has been deleted, reset the settings.
	if len(snaps) == 0 {
		input.Values.Set("cniCilium.internal.hubble.settings", internal.HubbleMonitoringConfigSpec{
			ExtendedMetrics: internal.ExtendedMetricsSpec{
				Enabled:    false,
				Collectors: make([]internal.ExtendedMetricCollector, 0),
			},
			FlowLogs: internal.FlowLogsSpec{
				Enabled:         false,
				AllowFilterList: []*internal.FlowLogFilter{},
				DenyFilterList:  []*internal.FlowLogFilter{},
				FieldMaskList:   []internal.FlowLogFieldMask{},
				FileMaxSizeMB:   10,
			},
		})
		return nil
	}
	if len(snaps) > 1 {
		return fmt.Errorf("multiple snapshots found for %q", hubbleMonitoringConfigSnapshotName)
	}

	var spec internal.HubbleMonitoringConfigSpec
	if err := snaps[0].UnmarshalTo(&spec); err != nil {
		return fmt.Errorf("cannot unmarshal spec from snapshot: %w", err)
	}

	input.Values.Set("cniCilium.internal.hubble.settings", spec)

	return nil
}
