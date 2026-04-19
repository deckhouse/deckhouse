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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// nodeTarget is a piece of configuration for ping exporter. It represents a single node instance.
type nodeTarget struct {
	Name    string `json:"name"`
	Address string `json:"ipAddress"`
}

//nolint:unused //TODO: fix unused linters
type targets struct {
	Cluster []nodeTarget `json:"clusterTargets"`
}

func getAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	if node.Spec.Unschedulable {
		return nil, nil
	}
	target := nodeTarget{Name: node.Name}
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			target.Address = address.Address
			break
		}
	}

	return target, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/monitoring-ping/discover_targets",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "addresses",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: getAddress,
		},
	},
}, updateNodeList)

func updateNodeList(_ context.Context, input *go_hook.HookInput) error {
	lenSnapshot := len(input.Snapshots.Get("addresses"))
	nodes := make([]nodeTarget, 0, lenSnapshot)

	for nt, err := range sdkobjectpatch.SnapshotIter[nodeTarget](input.Snapshots.Get("addresses")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'addresses' snapshots: %w", err)
		}

		if nt.Address != "" {
			nodes = append(nodes, nt)
		}
	}

	input.Values.Set("monitoringPing.internal.clusterTargets", nodes)

	return nil
}
