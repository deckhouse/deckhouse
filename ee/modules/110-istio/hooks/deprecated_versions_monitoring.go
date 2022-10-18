/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

var (
	versionsMonitoringMetricsGroup = "versions"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        internal.Queue(versionsMonitoringMetricsGroup),
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, versionMonitoringHook)

func versionMonitoringHook(input *go_hook.HookInput) error {
	if !input.Values.Get("istio.globalVersion").Exists() {
		return nil
	}

	input.MetricsCollector.Expire(versionsMonitoringMetricsGroup)
	globalVersion := input.Values.Get("istio.globalVersion").String()
	additionalVersions := input.Values.Get("istio.additionalVersions").Array()
	deprecatedVersions := input.Values.Get("istio.internal.deprecatedVersions").Array()
	istioVersionsMap := make(map[string]struct{}, 0)

	istioVersionsMap[globalVersion] = struct{}{}
	for _, additionalVersion := range additionalVersions {
		istioVersionsMap[additionalVersion.String()] = struct{}{}
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
