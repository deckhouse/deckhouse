/*
Copyright 2022 Flant JSC

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
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

type nodeKernelVersion struct {
	Name          string
	KernelVersion string
}

const (
	ciliumConstraint         = ">= 4.9.17"
	ciliumAndIstioConstraint = ">= 5.7"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: filterNodes,
		},
	},
}, handleNodes)

func filterNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	return nodeKernelVersion{
		Name:          node.Name,
		KernelVersion: node.Status.NodeInfo.KernelVersion,
	}, nil
}

func handleNodes(input *go_hook.HookInput) error {
	snap := input.Snapshots["nodes"]
	for _, n := range snap {
		node := n.(nodeKernelVersion)
		v, err := semver.NewVersion(strings.Split(node.KernelVersion, "-")[0])
		if err != nil {
			return fmt.Errorf("cannot parse kernel version %s for node %s: %v", node.KernelVersion, node.Name, err)
		}
		enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

		// check kernel requirements for cilium module
		if enabledModules.Has("cni-cilium") {
			c, err := semver.NewConstraint(ciliumConstraint)
			if err != nil {
				return err
			}
			if !c.Check(v) {
				input.MetricsCollector.Set("d8_node_kernel_does_not_satisfy_requirements", 1, map[string]string{"name": node.Name, "kernel_version": node.KernelVersion, "module": "cilium", "constraint": ciliumConstraint})
				return fmt.Errorf("kernel %s on node %s does not satisfy cilium kernel constraint %s", node.KernelVersion, node.Name, ciliumConstraint)
			}

			// check kernel requirements for cilium and istio
			if enabledModules.Has("istio") {
				c, err := semver.NewConstraint(ciliumAndIstioConstraint)
				if err != nil {
					return err
				}
				if !c.Check(v) {
					input.MetricsCollector.Set("d8_node_kernel_does_not_satisfy_requirements", 1, map[string]string{"name": node.Name, "kernel_version": node.KernelVersion, "module": "cilium,istio", "constraint": ciliumAndIstioConstraint})
					return fmt.Errorf("kernel %s on node %s does not satisfy cilium+istio kernel constraint %s", node.KernelVersion, node.Name, ciliumAndIstioConstraint)
				}
			}
		}
	}

	return nil
}
