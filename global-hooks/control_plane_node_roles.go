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
	"fmt"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	masterNodeRole       = "node-role.kubernetes.io/master"
	controlPlaneNodeRole = "node-role.kubernetes.io/control-plane"
)

var (
	roleLabelsPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				masterNodeRole:       "",
				controlPlaneNodeRole: "",
			},
		},
	}
)

// This hook adds node-role.kubernetes.io/control-plane label to all nodes with
// node-role.kubernetes.io/master label. And vice versa.
func applyBothNodeRoles(input *go_hook.HookInput) error {
	masters, err := sdkobjectpatch.UnmarshalToStruct[labeledNode](input.NewSnapshots, "master_nodes")
	if err != nil {
		return fmt.Errorf("unmarshal master_nodes: %w", err)
	}

	controlPlanes, err := sdkobjectpatch.UnmarshalToStruct[labeledNode](input.NewSnapshots, "control_plane_nodes")
	if err != nil {
		return fmt.Errorf("unmarshal control_plane_nodes: %w", err)
	}

	allNodes := append(masters, controlPlanes...)

	for _, node := range allNodes {
		if node.MasterLabelExists && node.ControlPlaneLabelExists {
			continue
		}

		input.PatchCollector.MergePatch(roleLabelsPatch, "v1", "Node", "", node.Name)
	}

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
			FilterFunc: filterLabeledNode,
		},
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      masterNodeRole,
				Operator: metav1.LabelSelectorOpExists,
			}}},
			FilterFunc: filterLabeledNode,
		},
	},
}, applyBothNodeRoles)

func filterLabeledNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()

	_, masterLabelExists := labels[masterNodeRole]
	_, controlPlaneLabelExists := labels[controlPlaneNodeRole]

	return labeledNode{
		Name:                    obj.GetName(),
		MasterLabelExists:       masterLabelExists,
		ControlPlaneLabelExists: controlPlaneLabelExists,
	}, nil
}

type labeledNode struct {
	Name                    string
	MasterLabelExists       bool
	ControlPlaneLabelExists bool
}
