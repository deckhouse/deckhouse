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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

type ReservedNode struct {
	Name                string
	UsedLabelsAndTaints []string
}

func applyLabelTaintFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	n := ReservedNode{Name: node.Name}
	usedLabelsAndTaints := set.New()

	for _, taint := range node.Spec.Taints {
		if taint.Key == "dedicated.deckhouse.io" {
			usedLabelsAndTaints.Add(taint.Value)
			break
		}
	}

	for k := range node.ObjectMeta.Labels {
		if strings.HasPrefix(k, "node-role.deckhouse.io/") {
			usedLabelsAndTaints.Add(strings.Split(k, "/")[1])
		}
	}

	n.UsedLabelsAndTaints = usedLabelsAndTaints.Slice()

	return n, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-custom",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyLabelTaintFilter,
		},
	},
}, exposeDomainNodes)

func checkLabelsAndTaints(labelsAndTaints []string, modules set.Set) bool {
	for _, labelOrTaint := range labelsAndTaints {
		matched := modules.Has(labelOrTaint)
		if !matched {
			return !matched
		}
	}
	return false
}

func exposeDomainNodes(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("")

	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

	// Adding reserved names
	enabledModules.Add("monitoring", "system", "frontend")

	nodes := input.Snapshots["nodes"]

	for _, o := range nodes {
		node := o.(ReservedNode)
		if checkLabelsAndTaints(node.UsedLabelsAndTaints, enabledModules) {
			input.MetricsCollector.Set(
				"reserved_domain_nodes",
				1.0,
				map[string]string{
					"name": node.Name,
				},
			)
		}
	}
	return nil
}
