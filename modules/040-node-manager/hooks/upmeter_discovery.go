/*
Copyright 2022 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/apis/nodegroups/v1"
)

// filterDynamicProbeNodeGroups returns the name of a nodegroup to consider or emptystring if it should be skipped
func filterDynamicProbeNodeGroups(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return "", err
	}

	// Filter only cloud node groups
	if nodeGroup.Spec.NodeType != ngv1.NodeTypeCloudEphemeral {
		return "", nil
	}

	// Filter node groups that can violate availability
	minPerZone := nodeGroup.Spec.CloudInstances.MinPerZone
	if minPerZone == nil || *minPerZone < 1 {
		return "", nil
	}

	return obj.GetName(), nil
}

// This hook discovers nodegroup names for dynamic probes in upmeter
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue:        "/modules/node-manager",
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "nodegroups",
				ApiVersion: "deckhouse.io/v1",
				Kind:       "NodeGroup",
				FilterFunc: filterDynamicProbeNodeGroups,
			},
		},
	},
	collectDynamicProbeConfig,
)

// collectDynamicProbeConfig sets names of objects to internal values
func collectDynamicProbeConfig(input *go_hook.HookInput) error {
	// Input
	var (
		key   = "nodeManager.internal.upmeterDiscovery.ephemeralNodeGroupNames"
		names = parseNames(input.Snapshots["nodegroups"])
	)

	// Output
	input.Values.Set(key, names)
	return nil
}

// parseNames parses filter string result to a sorted strings slice
func parseNames(results []go_hook.FilterResult) []string {
	s := set.New()
	for _, name := range results {
		s.Add(name.(string))
	}
	s.Delete("") // throw away invalid ones
	return s.Slice()
}
