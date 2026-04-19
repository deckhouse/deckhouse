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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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
func applyBothNodeRoles(_ context.Context, input *go_hook.HookInput) error {
	nodes := make([]labeledNode, 0)
	for _, snapshotName := range []string{"master_nodes", "control_plane_nodes"} {
		snapshots, err := sdkobjectpatch.UnmarshalToStruct[labeledNode](input.Snapshots, snapshotName)
		if err != nil {
			return fmt.Errorf("failed to unmarshal %s snapshot: %w", snapshotName, err)
		}

		nodes = append(nodes, snapshots...)
	}

	for _, node := range nodes {
		if node.MasterLabelExists && node.ControlPlaneLabelExists {
			continue
		}

		input.PatchCollector.PatchWithMerge(roleLabelsPatch, "v1", "Node", "", node.Name)
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
