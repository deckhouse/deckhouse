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
)

const (
	deprecatedZone = "ru-central1-c"
	metricName     = "d8_cloud_provider_yandex_nat_instance_zone_deprecated"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 21},
}, alertOnDeprecatedNatInstanceZone)

func alertOnDeprecatedNatInstanceZone(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricName)

	layout, ok := input.Values.GetOk("cloudProviderYandex.internal.providerClusterConfiguration.layout")
	if !ok {
		input.Logger.Warn("No providerClusterConfiguration values received, skipping zone check for NAT Instance")
		return nil
	}
	if layout.String() != "WithNATInstance" {
		return nil
	}

	natInstanceName, ok := input.Values.GetOk("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName")
	if !ok {
		input.Logger.Warn("No natInstanceName value received, skipping zone check for NAT Instance")
		return nil
	}
	natInstanceZone, ok := input.Values.GetOk("cloudProviderYandex.internal.providerDiscoveryData.natInstanceZone")
	if !ok {
		input.Logger.Warn("No natInstanceZone value received, skipping zone check for NAT Instance")
		return nil
	}

	if natInstanceZone.String() != deprecatedZone {
		return nil
	}

	input.MetricsCollector.Set(metricName, 1, map[string]string{
		"name": natInstanceName.String(),
		"zone": natInstanceZone.String(),
	}, metrics.WithGroup(metricName))

	return nil
}
