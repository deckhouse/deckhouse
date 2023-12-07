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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	deprecatedZone = "ru-central1-c"
	metricName     = "d8_cloud_provider_yandex_nat_instance_zone_deprecated"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deprecatedNatInstanceZone",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			FilterFunc: cloudProviderDiscoveryDataFromSecret,
		},
	},
}, alertOnDeprecatedNatInstanceZone)

func cloudProviderDiscoveryDataFromSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert Kubernetes secret to secret struct: %v", err)
	}

	return secret.Data["cloud-provider-discovery-data.json"], nil
}

func alertOnDeprecatedNatInstanceZone(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricName)

	layout, ok := input.Values.GetOk("cloudProviderYandex.internal.providerClusterConfiguration.layout")
	if !ok {
		input.LogEntry.Warn("No providerClusterConfiguration values received, skipping zone check for NAT Instance")
		return nil
	}
	if layout.String() != "WithNATInstance" {
		return nil
	}

	natInstanceName, ok := input.Values.GetOk("cloudProviderYandex.internal.providerDiscoveryData.natInstanceName")
	if !ok {
		input.LogEntry.Warn("No natInstanceName value received, skipping zone check for NAT Instance")
		return nil
	}
	natInstanceZone, ok := input.Values.GetOk("cloudProviderYandex.internal.providerDiscoveryData.natInstanceZone")
	if !ok {
		input.LogEntry.Warn("No natInstanceZone value received, skipping zone check for NAT Instance")
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
