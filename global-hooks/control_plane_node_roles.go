// Copyright 2022 Flant JSC
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

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	masterNodeRole       = "node-role.kubernetes.io/master"
	controlPlaneNodeRole = "node-role.kubernetes.io/control-plane"
)

// This hook adds node-role.kubernetes.io/control-plane label to all nodes with
// node-role.kubernetes.io/master label. And vice versa.
func applyBothNodeRoles(input *go_hook.HookInput) error {
	applyNodeRole(input, set.NewFromSnapshot(input.Snapshots["control_plane_nodes"]), masterNodeRole)
	applyNodeRole(input, set.NewFromSnapshot(input.Snapshots["master_nodes"]), controlPlaneNodeRole)
	return nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "control_plane_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      controlPlaneNodeRole,
				Operator: metav1.LabelSelectorOpExists,
			}}},
			FilterFunc: filterName,
		},
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      masterNodeRole,
				Operator: metav1.LabelSelectorOpExists,
			}}},
			FilterFunc: filterName,
		},
	},
}, applyBothNodeRoles)

func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func applyNodeRole(input *go_hook.HookInput, names set.Set, label string) {
	for _, name := range names.Slice() {
		input.PatchCollector.Filter(getLabelPatch(label), "v1", "Node", "", name)
	}
}

func getLabelPatch(label string) func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		node := new(v1.Node)
		err := sdk.FromUnstructured(obj, node)
		if err != nil {
			return nil, err
		}

		node.Labels[label] = ""

		return sdk.ToUnstructured(node)
	}
}
