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

package fencing

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const deprecatedSettingsMetric = "d8_node_group_fencing_deprecated_settings"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:      "/modules/node-manager/fencing",
	Kubernetes: []go_hook.KubernetesConfig{nodeGroupsBinding},
}, handleDeprecatedSettings)

// handleDeprecatedSettings publishes one series for every deprecated fencing setting that a
// NodeGroup still uses. It expires the group on every run, so a series disappears
// when the setting is removed.
func handleDeprecatedSettings(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(deprecatedSettingsMetric)

	set := func(name, setting string) {
		input.MetricsCollector.Set(deprecatedSettingsMetric, 1,
			map[string]string{"name": name, "setting": setting},
			metrics.WithGroup(deprecatedSettingsMetric))
	}

	for ng, err := range sdkobjectpatch.SnapshotIter[nodeGroupFencing](input.Snapshots.Get(nodeGroupsSnapshot)) {
		if err != nil {
			return fmt.Errorf("failed to iterate over '%s' snapshots: %w", nodeGroupsSnapshot, err)
		}
		if ng.Mode == "Watchdog" {
			set(ng.Name, "mode")
		}
		if ng.WatchdogTimeout != "" {
			set(ng.Name, "watchdog.timeout")
		}
	}

	return nil
}
