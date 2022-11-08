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

/*
This hook checks nodes kernel requirements and set internal flag stopMainQueue.
This flag used in another hook, stop_main_queue.go, which stops main queue if flag is true.
We cannot stop queue in this hook, because we loose metrics if hook fails.
*/

package hooks

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

type nodeKernelVersion struct {
	Name          string
	KernelVersion string
	SemverVersion *semver.Version
}

type nodeConstraint struct {
	KernelVersionConstraint string
	ModulesListInUse        []string
}

var constraints = []nodeConstraint{
	{
		KernelVersionConstraint: ">= 4.9.17",
		ModulesListInUse:        []string{"cni-cilium"},
	},
	{
		KernelVersionConstraint: ">= 5.7",
		ModulesListInUse:        []string{"cni-cilium", "istio"},
	},
	{
		KernelVersionConstraint: ">= 5.7",
		ModulesListInUse:        []string{"cni-cilium", "openvpn"},
	},
	{
		KernelVersionConstraint: ">= 5.7",
		ModulesListInUse:        []string{"cni-cilium", "node-local-dns"},
	},
}

const (
	nodeKernelCheckMetricsGroup = "node_kernel_check"
	nodeKernelCheckMetricName   = "d8_node_kernel_does_not_satisfy_requirements"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
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
	/* Kernel version should be splitted to parts because versions `5.15.0-52-generic`
	parses by semver as prerelease version. Prerelease versions by default come before stable versions
	in the order of precedence, so in semver terms `5.15.0-52-generic` less than `5.15`.
	More info - https://github.com/Masterminds/semver#working-with-prerelease-versions */
	v, err := semver.NewVersion(strings.Split(node.Status.NodeInfo.KernelVersion, "-")[0])
	if err != nil {
		return nil, fmt.Errorf("cannot parse kernel version %s for node %s: %v", node.Status.NodeInfo.KernelVersion, node.Name, err)
	}

	return nodeKernelVersion{
		Name:          node.Name,
		KernelVersion: node.Status.NodeInfo.KernelVersion,
		SemverVersion: v,
	}, nil
}

func handleNodes(input *go_hook.HookInput) error {
	var hasAffectedNodes bool

	input.MetricsCollector.Expire(nodeKernelCheckMetricsGroup)

	snap := input.Snapshots["nodes"]
	if len(snap) == 0 {
		return nil
	}

	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

	for _, constrant := range constraints {
		// check modules in use
		check := true
		for _, m := range constrant.ModulesListInUse {
			if !enabledModules.Has(m) {
				check = false
				break
			}
		}
		if !check {
			continue
		}

		c, err := semver.NewConstraint(constrant.KernelVersionConstraint)
		if err != nil {
			return err
		}

		for _, n := range snap {
			node := n.(nodeKernelVersion)

			if !c.Check(node.SemverVersion) {
				modulesListInUse := strings.Join(constrant.ModulesListInUse, ",")
				input.MetricsCollector.Set(nodeKernelCheckMetricName, 1, map[string]string{
					"node":            node.Name,
					"kernel_version":  node.KernelVersion,
					"affected_module": modulesListInUse,
					"constraint":      constrant.KernelVersionConstraint,
				}, metrics.WithGroup(nodeKernelCheckMetricsGroup))
				input.LogEntry.Debugf("kernel %s on node %s does not satisfy kernel constraint %s for modules [%s]", node.KernelVersion, node.Name, constrant.KernelVersionConstraint, modulesListInUse)
				hasAffectedNodes = true
			}
		}
	}

	if hasAffectedNodes {
		input.LogEntry.Error("some nodes have unmet kernel constraints. To observe affected nodes use the expr `d8_node_kernel_does_not_satisfy_requirements == 1` in Prometheus")
	}

	return nil
}
