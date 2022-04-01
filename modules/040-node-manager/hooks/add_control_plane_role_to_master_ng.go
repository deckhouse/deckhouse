// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

// TODO remove after 1.33 release
// add control-plane role and "node.kubernetes.io/exclude-from-external-load-balancers" label
// for all master nodes over master node group
// At current moment, first bootstrapped master get 'control-plane' role,
// but other master nodes don't get this role, because
// first master bootstrapped with kubeadm (kubeadm set role to node over label), but
// master node group was created on bootstrap does not contain label with role
// we will add label to nodegroup template for existing clusters

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	controlPlaneRoleLabel    = "node-role.kubernetes.io/control-plane"
	excludeLoadBalancerLabel = "node.kubernetes.io/exclude-from-external-load-balancers"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                "master_nodes_with_external_lb",
			ExecuteHookOnEvents: pointer.BoolPtr(false),
			ApiVersion:          "v1",
			Kind:                "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/master": "",
					excludeLoadBalancerLabel:         "",
				},
			},
			FilterFunc: nodeWithExternalLBLabel,
		},
	},
}, relabelControlPlaneNodes)

func nodeWithExternalLBLabel(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func relabelControlPlaneNodes(input *go_hook.HookInput) error {
	// we have corner case
	// all clusters on EarlyAccess and lower have label in NodeGroup nodeTemplate
	// and removing label from NodeGroup nodeTemplate remove label from node
	// BUT clusters on Stable and RockSolid do not have in NodeGroup nodeTemplate,
	// but have on first control-plane node which bootstrapped with kubeadm
	// we should remove label from node manually
	nodePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				excludeLoadBalancerLabel: nil,
			},
		},
	}

	for _, name := range set.NewFromSnapshot(input.Snapshots["master_nodes_with_external_lb"]).Slice() {
		input.PatchCollector.MergePatch(nodePatch, "v1", "Node", "", name, object_patch.IgnoreMissingObject())
	}

	ngPatch := map[string]interface{}{
		"spec": map[string]interface{}{
			"nodeTemplate": map[string]interface{}{
				"labels": map[string]interface{}{
					controlPlaneRoleLabel:    "",
					excludeLoadBalancerLabel: nil,
				},
			},
		},
	}

	input.PatchCollector.MergePatch(ngPatch, "deckhouse.io/v1", "NodeGroup", "", "master", object_patch.IgnoreMissingObject())

	return nil
}
