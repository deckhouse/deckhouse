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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

/*
TODO(kubernetesVersion-deprecation): T+1 remove — delete this hook with the
D8UnsetKubernetesVersionInModuleConfig alert (mc-migration.yaml + deckhouse-alerts.yml)
once ModuleConfig carries kubernetesVersion across the fleet.

Description:
	Alerts when ModuleConfig control-plane-manager has no kubernetesVersion setting.
*/

// TODO(kubernetesVersion-deprecation): T+1 measure — adoption counter on unset ModuleConfig
// kubernetesVersion; read fleet-wide before deciding on any release-side enforcement.
const (
	unsetKubernetesVersionMetricGroup = "D8UnsetKubernetesVersionInModuleConfig"
	unsetKubernetesVersionMetricName  = "d8_unset_kubernetes_version_in_module_config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/control-plane-manager/alerting",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, checkKubernetesVersionMigration)

func checkKubernetesVersionMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(unsetKubernetesVersionMetricGroup)

	mcVersion := input.Values.Get("controlPlaneManager.kubernetesVersion").String()
	migrated := isKubernetesVersionMigrated(mcVersion)

	if !migrated {
		input.MetricsCollector.Set(
			unsetKubernetesVersionMetricName, 1,
			map[string]string{},
			metrics.WithGroup(unsetKubernetesVersionMetricGroup),
		)
	}

	return nil
}

// isKubernetesVersionMigrated reports whether ModuleConfig control-plane-manager has
// kubernetesVersion set. Presence (including the deprecated Automatic alias) is enough — the
// alert targets only clusters that still rely on the unset→CC-fallback path.
func isKubernetesVersionMigrated(mcVersion string) bool {
	return mcVersion != ""
}
