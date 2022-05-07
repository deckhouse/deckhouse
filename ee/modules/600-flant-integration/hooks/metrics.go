/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// this hook checks flant integration configuration
// hook returns only metric:
//   `d8_flant_integration_misconfiguration_detected`:
//      0 - is ok
//      1 - madison integration is enabled but metrics shipment is disabled
//      2 - madison integration and metrics shipment are enabled but kubeall host is not set
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/flant-integration/metrics",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, handleValues)

func handleValues(input *go_hook.HookInput) error {
	madisonAuthKey := input.Values.Get("flantIntegration.madisonAuthKey").String()
	metrics := input.Values.Get("flantIntegration.metrics.url").String()
	kubeallHostIsSet := input.Values.Get("flantIntegration.kubeall.host").Exists()
	value := 0
	if madisonAuthKey != "false" {
		if metrics != "https://connect.deckhouse.io/v1/remote_write" {
			value = 1
		} else {
			if !kubeallHostIsSet {
				value = 2
			}
		}
	}

	input.MetricsCollector.Set("d8_flant_integration_misconfiguration_detected", float64(value), map[string]string{})
	return nil
}
