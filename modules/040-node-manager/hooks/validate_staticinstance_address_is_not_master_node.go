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

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NodeInfoAddress struct {
	Address string
}

type StaticInstance struct {
	Address string `json:"address"`
}

func nodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	var address string
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			address = addr.Address
			break
		}
		if addr.Type == v1.NodeExternalIP && address == "" {
			address = addr.Address
		}
	}

	return NodeInfoAddress{
		Address: address,
	}, nil
}

func staticInstanceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	address, found, err := unstructured.NestedString(obj.Object, "spec", "address")
	if err != nil || !found || address == "" {
		return nil, fmt.Errorf("failed to get address from StaticInstance: %v", err)
	}

	return StaticInstance{
		Address: address,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/validate_static_instances",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: nodeFilter,
		},
		{
			Name:       "static_instances",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "StaticInstance",
			FilterFunc: staticInstanceFilter,
		},
	},
}, validateStaticInstanceAddresses)

func validateStaticInstanceAddresses(_ context.Context, input *go_hook.HookInput) error {
	nodeSnapshots := input.Snapshots.Get("nodes")
	staticInstanceSnapshots := input.Snapshots.Get("static_instances")

	if len(staticInstanceSnapshots) == 0 || len(nodeSnapshots) == 0 {
		return nil
	}

	masterAddresses := make(map[string]struct{})
	for nodeInfo, err := range sdkobjectpatch.SnapshotIter[NodeInfo](nodeSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes snapshots': %v", err)
		}
		if nodeInfo.Address == "" {
			return fmt.Errorf("master node has empty address")
		}
		masterAddresses[nodeInfo.Address] = struct{}{}
	}

	for staticInstance, err := range sdkobjectpatch.SnapshotIter[StaticInstance](staticInstanceSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'static_instances snapshots': %v", err)
		}

		// Check if static instance address matches any master node address
		if _, exists := masterAddresses[staticInstance.Address]; exists {
			return fmt.Errorf("static instance address %s conflicts with master node address", staticInstance.Address)
		}
	}

	return nil
}
