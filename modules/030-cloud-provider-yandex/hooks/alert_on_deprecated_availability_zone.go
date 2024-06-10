/*
Copyright 2024 Flant JSC

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
	"net/url"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	yandexDeprecatedZoneNodesKey   = "node_groups_with_deprecated_region"
	yandexDeprecatedZoneInNodesKey = "yandex:hasDeprecatedZoneInNodes"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       yandexDeprecatedZoneNodesKey,
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"topology.kubernetes.io/zone": "ru-central1-c",
				},
			},
			FilterFunc: filterYandexDeprecatedZoneNodes,
		},
	},
}, alertOnNodesInDeprecatedAvailabilityZones)

func filterYandexDeprecatedZoneNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	if err := sdk.FromUnstructured(obj, node); err != nil {
		return nil, err
	}

	providerID, err := url.Parse(node.Spec.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("Parse Node's .spec.providerID: %w", err)
	}
	if providerID.Scheme != "yandex" {
		return nil, nil
	}

	return node.Labels["node.deckhouse.io/group"], nil
}

func alertOnNodesInDeprecatedAvailabilityZones(input *go_hook.HookInput) error {
	nodeGroupsWithDeprecatedZones := set.NewFromSnapshot(input.Snapshots[yandexDeprecatedZoneNodesKey])

	if len(nodeGroupsWithDeprecatedZones) > 0 {
		requirements.SaveValue(yandexDeprecatedZoneInNodesKey, true)
	} else {
		requirements.SaveValue(yandexDeprecatedZoneInNodesKey, false)
	}

	for _, nodeGroupName := range nodeGroupsWithDeprecatedZones.Slice() {
		input.MetricsCollector.Set("d8_node_group_node_with_deprecated_availability_zone", 1, map[string]string{
			"node_group": nodeGroupName,
		})
	}

	return nil
}
