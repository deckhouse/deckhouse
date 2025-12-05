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
			Name:       "etcd_arbiter_node",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node.deckhouse.io/etcd-arbiter": "",
				},
			},
			FilterFunc: applyNodeFilter,
		},
	},
}, handleCheckEtcdArbiterNode)

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleCheckEtcdArbiterNode(_ context.Context, input *go_hook.HookInput) error {
	etcdArbiterNodes := input.Snapshots.Get("etcd_arbiter_node")

	if len(etcdArbiterNodes) > 1 {
		return fmt.Errorf("etcd-arbiter label must be present only for one node, found %d nodes", len(etcdArbiterNodes))
	}

	input.Values.Set("controlPlaneManager.internal.hasEtcdArbiterNode", len(etcdArbiterNodes) == 1)

	return nil
}
