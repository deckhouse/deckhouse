/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

const (
	deprecatedModuleParamMetricName             = "d8_istio_deprecated_module_param"
	deprecatedModuleParamMonitoringMetricsGroup = "deprecated_module_param"
	istioTLSModePath                            = "istio.tlsMode"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, getRidOfDeprecatedParams)

func getRidOfDeprecatedParams(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(deprecatedModuleParamMonitoringMetricsGroup)
	if input.ConfigValues.Exists(istioTLSModePath) {
		labels := map[string]string{
			"param": "tlsMode",
		}
		input.MetricsCollector.Set(deprecatedModuleParamMetricName, 1, labels, metrics.WithGroup(deprecatedModuleParamMonitoringMetricsGroup))
	}
	return nil
}
