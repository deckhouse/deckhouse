/*
Copyright 2025 Flant JSC

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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: applyNodeFilter,
		},
		{
			Name:       "etcd_only_node",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.deckhouse.io/etcd-only": "",
				},
			},
			FilterFunc: applyNodeFilter,
		},
	},
}, handleCheckEtcdOnlyNode)

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleCheckEtcdOnlyNode(_ context.Context, input *go_hook.HookInput) error {
	masterNodes := input.Snapshots.Get("master_nodes")
	etcdOnlyNodes := input.Snapshots.Get("etcd_only_node")

	if len(etcdOnlyNodes) > 1 {
		return fmt.Errorf("etcd-only label must be present on at most one node, found %d nodes", len(etcdOnlyNodes))
	}

	hasEtcdOnlyNode := len(masterNodes) == 2 && len(etcdOnlyNodes) == 1

	input.Values.Set("controlPlaneManager.internal.hasEtcdOnlyNode", hasEtcdOnlyNode)

	return nil
}
