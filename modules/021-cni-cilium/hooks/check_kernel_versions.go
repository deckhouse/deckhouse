/*
Copyright 2024 Flant JSC

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
This hook checks if node kernel versions satisfy the requirements
for the currently enabled modules. If not, it sets a metric indicating
non-compliance. Another hook (stop_main_queue.go) reads this information
to decide whether to stop the main processing queue

We do not stop the queue directly in this hook to avoid losing metrics
in case of hook failure
*/

package hooks

import (
	"log/slog"
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

// Requirement defines a minimum kernel version constraint for a set of modules
type nodeConstraint struct {
	kernelVersionConstraint string
	modulesListInUse        []string
}

// List of requirements to be checked against node kernel versions.
var constraints = []nodeConstraint{
	{
		kernelVersionConstraint: ">= 4.9.17",
		modulesListInUse:        []string{"cni-cilium"},
	},
	{
		kernelVersionConstraint: ">= 5.7",
		modulesListInUse:        []string{"cni-cilium", "istio"},
	},
	{
		kernelVersionConstraint: ">= 5.7",
		modulesListInUse:        []string{"cni-cilium", "openvpn"},
	},
	{
		kernelVersionConstraint: ">= 5.7",
		modulesListInUse:        []string{"cni-cilium", "node-local-dns"},
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

// nodeKernelVersion contains node name and kernel version string
type nodeKernelVersion struct {
	Name          string
	KernelVersion string
}

// filterNodes extracts kernel version and node name from a Node object
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

// handleNodes is the main hook logic that checks kernel requirements and emits metrics
func handleNodes(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(nodeKernelCheckMetricsGroup)

	nodes := input.Snapshots["nodes"]
	if len(nodes) == 0 {
		input.Logger.Error("no nodes found")
		return nil
	}

	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

	// Iterate through all defined kernel requirements
	for _, constraint := range constraints {
		check := true
		for _, m := range constraint.modulesListInUse {
			if !enabledModules.Has(m) {
				check = false
				break
			}
		}
		if !check {
			continue
		}

		// Parse the kernel version constraint
		c, err := semver.NewConstraint(constraint.kernelVersionConstraint)
		if err != nil {
			return err
		}

		// Set minimal required kernel version constraint in .Values
		input.Values.Set("cniCilium.internal.kernelVersionConstraint", constraint.kernelVersionConstraint)

		// Check each node's kernel version
		for _, n := range nodes {
			node := n.(nodeKernelVersion)

			modulesListInUse := strings.Join(constraint.modulesListInUse, ",")

			// Kernel versions like `5.15.0-52-generic` are considered pre-release in semver,
			// so we strip the suffix for accurate comparison
			nodeSemverVersion, err := semver.NewVersion(strings.Split(node.KernelVersion, "-")[0])
			if err != nil {
				input.Logger.Errorf("failed to parse kernel version %s: %v", node.KernelVersion, err)
				continue
			}

			// If node kernel does not satisfy the constraint, emit a metric and log error message
			if !c.Check(nodeSemverVersion) {
				input.MetricsCollector.Set(
					nodeKernelCheckMetricName,
					1,
					map[string]string{
						"node":            node.Name,
						"kernel_version":  node.KernelVersion,
						"affected_module": modulesListInUse,
						"constraint":      constraint.kernelVersionConstraint,
					},
					metrics.WithGroup(nodeKernelCheckMetricsGroup),
				)

				input.Logger.Error(
					"kernel on node does not satisfy kernel constraint for modules",
					slog.String("kernel_version", node.KernelVersion),
					slog.String("node", node.Name),
					slog.String("constraint", constraint.kernelVersionConstraint),
					slog.String("modules", modulesListInUse),
				)
			}
		}
	}

	return nil
}
