/*
Copyright 2023 Flant JSC

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

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

var (
	versionsMonitoringMetricsGroup = "versions"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        lib.Queue("monitoring"),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10}, // The hook relies on operatorVersionsToInstall value discovered in discovery_operator_versions_to_install.go before.
}, versionMonitoringHook)

func versionMonitoringHook(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get("istio.internal.operatorVersionsToInstall").Exists() {
		return nil
	}

	input.MetricsCollector.Expire(versionsMonitoringMetricsGroup)
	istioVersions := input.Values.Get("istio.internal.operatorVersionsToInstall").Array()
	deprecatedVersions := input.Values.Get("istio.internal.deprecatedVersions").Array()
	istioVersionsMap := make(map[string]struct{}, 0)

	for _, istioVersion := range istioVersions {
		istioVersionsMap[istioVersion.String()] = struct{}{}
	}

	for _, deprecatedVersion := range deprecatedVersions {
		if _, ok := istioVersionsMap[deprecatedVersion.Get("version").String()]; ok {
			labels := map[string]string{
				"version":        deprecatedVersion.Get("version").String(),
				"alert_severity": deprecatedVersion.Get("alertSeverity").String(),
			}
			input.MetricsCollector.Set("d8_istio_deprecated_version_installed", 1, labels, metrics.WithGroup(versionsMonitoringMetricsGroup))
		}
	}

	return nil
}
