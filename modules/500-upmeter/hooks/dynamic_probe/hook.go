/*
Copyright 2021 Flant JSC

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

package dynamic_probe

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// This hook populates internal values with object names that are used for dynamic probes in upmeter,
// and are rendered in its Helm templates.
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        "/modules/upmeter/dynamic_probes",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "upmeter_discovery_ingress_controllers",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"upmeter-discovery-controllers"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-ingress-nginx"},
					},
				},
				FilterFunc: filterNamesFromConfigmap,
			},
			{
				Name:       "upmeter_discovery_nodegroups",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"upmeter-discovery-cloud-ephemeral-nodegroups"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-cloud-instance-manager"},
					},
				},
				FilterFunc: filterNamesFromConfigmap,
			},
			{
				Name:       "cloud_provider_secret",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-node-manager-cloud-provider"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"kube-system"},
					},
				},
				FilterFunc: filterCloudProviderAvailabilityZonesFromSecret,
			},
		},
	},

	collectDynamicNames,
)

// collectDynamicNames sets names of objects to internal values
func collectDynamicNames(input *go_hook.HookInput) error {
	// Input, empty strings mean invalidated data
	var (
		ingressNames   = parseSingleStringSet(input.Snapshots["upmeter_discovery_ingress_controllers"]).Delete("").Slice()
		nodeGroupNames = parseSingleStringSet(input.Snapshots["upmeter_discovery_nodegroups"]).Delete("").Slice()
		cloudZones     = parseSingleStringSet(input.Snapshots["cloud_provider_secret"]).Delete("").Slice()
	)

	// Populate values
	data := emptyNames().WithIngressControllers(ingressNames...)
	if len(cloudZones) > 0 {
		// No zones, no scaling
		data = data.
			WithZones(cloudZones...).
			WithNodeGroups(nodeGroupNames...)
	}

	// Output
	input.Values.Set("upmeter.internal.dynamicProbes", data)
	return nil
}

func parseSingleStringSet(filtered []go_hook.FilterResult) set.Set {
	if len(filtered) == 0 {
		return set.New()
	}
	ss := filtered[0].([]string) // the secret MUST contain zones, so let it panic
	return set.New(ss...)
}

func filterNamesFromConfigmap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := new(v1.ConfigMap)
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, err
	}

	namesRaw, ok := cm.Data["names"]
	if !ok {
		return []string{}, nil
	}

	var names []string
	if err := yaml.Unmarshal([]byte(namesRaw), &names); err != nil {
		return nil, err
	}
	return names, nil
}

func filterCloudProviderAvailabilityZonesFromSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := new(v1.Secret)
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, err
	}

	data, ok := secret.Data["zones"]
	if !ok {
		// zone absence is fine for static clusters
		return []string{}, nil
	}

	var zones []string
	if err := yaml.Unmarshal(data, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}
