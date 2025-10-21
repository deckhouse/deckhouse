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
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type nodeConstraint struct {
	kernelVersionConstraint string
	modulesListInUse        []string
}

const (
	nodeKernelCheckMetricsGroup          = "node_kernel_check"
	nodeKernelCheckMetricName            = "d8_node_kernel_does_not_satisfy_requirements"
	minKernelVersionForExtraLBAlgorithms = "5.15"
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

type nodeKernelVersion struct {
	Name          string
	KernelVersion string
}

func filterNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, err
	}
	return nodeKernelVersion{
		Name:          node.Name,
		KernelVersion: node.Status.NodeInfo.KernelVersion,
	}, nil
}

func handleNodes(_ context.Context, input *go_hook.HookInput) error {
	constraints := []nodeConstraint{
		{
			kernelVersionConstraint: input.Values.Get("cniCilium.internal.minimalRequiredKernelVersionConstraint").String(),
			modulesListInUse:        []string{"cni-cilium"},
		},
	}
	extraLoadBalancerAlgorithmsEnabled := input.Values.Get("cniCilium.extraLoadBalancerAlgorithmsEnabled").Bool()
	if extraLoadBalancerAlgorithmsEnabled {
		currentConstraint := constraints[0].kernelVersionConstraint
		currentVersionStr := strings.TrimSpace(strings.TrimPrefix(currentConstraint, ">="))
		currentVersion, err := semver.NewVersion(currentVersionStr)
		if err != nil {
			return fmt.Errorf("failed to parse current version from constraint %q: %v", currentConstraint, err)
		}
		extraLBMinVersion, err := semver.NewVersion(minKernelVersionForExtraLBAlgorithms)
		if err != nil {
			return fmt.Errorf("invalid minKernelVersionForExtraLBAlgorithms %q: %v", minKernelVersionForExtraLBAlgorithms, err)
		}
		if extraLBMinVersion.GreaterThan(currentVersion) {
			constraints[0].kernelVersionConstraint = fmt.Sprintf(">= %s", extraLBMinVersion.String())
		}
	}

	input.MetricsCollector.Expire(nodeKernelCheckMetricsGroup)

	nodes := input.Snapshots.Get("nodes")
	if len(nodes) == 0 {
		input.Logger.Error("no nodes found")
		return nil
	}

	node, err := defineMinimalLinuxKernelVersionNode(nodes)
	if err != nil {
		input.Logger.Error("failed to define minimal kernel version node", log.Err(err))
		return nil
	}

	requirements.SaveValue("currentMinimalLinuxKernelVersion", node.KernelVersion)

	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")

	for _, constraint := range constraints {
		if !isConstraintRelevant(&enabledModules, constraint.modulesListInUse) {
			continue
		}

		c, err := semver.NewConstraint(constraint.kernelVersionConstraint)
		if err != nil {
			return fmt.Errorf("invalid kernel version constraint %q: %v", constraint.kernelVersionConstraint, err)
		}

		// Values is re-set to update the minimum required Linux kernel version depending on the included modules
		// The minimum version will later be passed to the cilium agent's cilium initContainer
		input.Values.Set("cniCilium.internal.minimalRequiredKernelVersionConstraint", constraint.kernelVersionConstraint)
		for node, err := range sdkobjectpatch.SnapshotIter[nodeKernelVersion](nodes) {
			if err != nil {
				return fmt.Errorf("failed to iterate over 'nodes' snapshots: %v", err)
			}

			kernelVerStr := strings.Split(node.KernelVersion, "-")[0]
			nodeSemverVersion, err := semver.NewVersion(kernelVerStr)
			if err != nil {
				input.Logger.Error("failed to parse kernel version", slog.String("version", node.KernelVersion), log.Err(err))
				continue
			}

			if !c.Check(nodeSemverVersion) {
				modulesList := strings.Join(constraint.modulesListInUse, ",")

				input.MetricsCollector.Set(
					nodeKernelCheckMetricName,
					1,
					map[string]string{
						"node":            node.Name,
						"kernel_version":  node.KernelVersion,
						"affected_module": modulesList,
						"constraint":      constraint.kernelVersionConstraint,
					},
					metrics.WithGroup(nodeKernelCheckMetricsGroup),
				)

				input.Logger.Error(
					"kernel on node does not satisfy kernel constraint for modules",
					slog.String("kernel_version", node.KernelVersion),
					slog.String("node", node.Name),
					slog.String("constraint", constraint.kernelVersionConstraint),
					slog.String("modules", modulesList),
				)
			}
		}
	}

	return nil
}

// Identify the cluster node (as nodeKernelVersion) with the lowest version of the kernel.
func defineMinimalLinuxKernelVersionNode(nodes []pkg.Snapshot) (*nodeKernelVersion, error) {
	var minimalNode *nodeKernelVersion
	var minimalVersion *semver.Version
	for node, err := range sdkobjectpatch.SnapshotIter[nodeKernelVersion](nodes) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over storage classes: %v", err)
		}

		kernelVerStr := strings.Split(node.KernelVersion, "-")[0]
		kernelVer, err := semver.NewVersion(kernelVerStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kernel version %q: %v", node.KernelVersion, err)
		}

		if minimalVersion == nil || kernelVer.LessThan(minimalVersion) {
			copied := node
			minimalNode = &copied
			minimalVersion = kernelVer
		}
	}

	return minimalNode, nil
}

// Check that module is constraint relevant
func isConstraintRelevant(enabled *set.Set, modules []string) bool {
	for _, m := range modules {
		if !enabled.Has(m) {
			return false
		}
	}
	return true
}
