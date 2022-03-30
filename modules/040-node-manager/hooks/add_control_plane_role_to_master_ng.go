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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	controlPlaneRoleLabel    = "node-role.kubernetes.io/control-plane"
	excludeLoadBalancerLabel = "node.kubernetes.io/exclude-from-external-load-balancers"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, addControlPlaneRoleToMasterNodeGroup)

func addControlPlaneRoleToMasterNodeGroup(input *go_hook.HookInput) error {
	input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		obj = obj.DeepCopy()

		labels, _, err := unstructured.NestedMap(obj.Object, "spec", "nodeTemplate", "labels")
		if err != nil {
			return nil, err
		}

		delete(labels, excludeLoadBalancerLabel)
		labels[controlPlaneRoleLabel] = ""

		err = unstructured.SetNestedMap(obj.Object, labels, "spec", "nodeTemplate", "labels")
		if err != nil {
			return nil, err
		}

		return obj, nil

	}, "deckhouse.io/v1", "NodeGroup", "", "master", object_patch.IgnoreMissingObject())

	return nil
}
