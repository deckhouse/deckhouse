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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
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
	var (
		minPerZone            = nodeGroup.Spec.CloudInstances.MinPerZone
		maxUnavailablePerZone = nodeGroup.Spec.CloudInstances.MaxUnavailablePerZone
	)
	if minPerZone == nil || *minPerZone < 1 {
		return "", nil
	}
	if maxUnavailablePerZone != nil && *minPerZone-*maxUnavailablePerZone < 1 {
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

type upmeterDiscovery struct {
	EphemeralNodeGroupNames []string `json:"ephemeralNodeGroupNames"`
}

// collectDynamicProbeConfig sets names of objects to internal values
func collectDynamicProbeConfig(_ context.Context, input *go_hook.HookInput) error {
	// Input
	key := "nodeManager.internal.upmeterDiscovery"
	parseNodeGroupNames, err := parseNames(input.Snapshots.Get("nodegroups"))
	if err != nil {
		return fmt.Errorf("failed to parse nodegroup names: %w", err)
	}

	discovery := upmeterDiscovery{
		EphemeralNodeGroupNames: parseNodeGroupNames,
	}

	// Output
	input.Values.Set(key, discovery)
	return nil
}

// parseNames parses filter string result to a sorted strings slice
func parseNames(results []pkg.Snapshot) ([]string, error) {
	s := set.New()

	for name, err := range sdkobjectpatch.SnapshotIter[string](results) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'nodegroups' snapshot: %w", err)
		}

		s.Add(name)
	}
	s.Delete("") // throw away invalid ones
	return s.Slice(), nil
}
