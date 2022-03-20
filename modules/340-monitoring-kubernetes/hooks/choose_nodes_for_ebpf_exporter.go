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
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
	ngHooks "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/api/v1"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

const (
	ebpfSchedulingLabelKey = "monitoring-kubernetes.deckhouse.io/ebpf-supported"
)

type NodeEligibility struct {
	Name            string
	NodeGroup       string
	IsEbpfSupported bool
}

var (
	kernelRegex              = regexp.MustCompile(`^(\d+\.\d+\.\d+).*$`)
	minSupportedKernelSemVer = semver.MustParse("5.4.0")
)

func getNodeNameWithSupportedDistro(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	nodeEligibility := &NodeEligibility{Name: node.Name, NodeGroup: node.Labels[ngHooks.NodeGroupNameLabel]}

	matches := kernelRegex.FindStringSubmatch(node.Status.NodeInfo.KernelVersion)
	if len(matches) != 2 {
		return nil, fmt.Errorf("failed to match kernel semver in %q with regex %q",
			node.Status.NodeInfo.KernelVersion, kernelRegex.String())
	}
	kernelSemVerStr := matches[1]

	kernelSemVer, err := semver.NewVersion(kernelSemVerStr)
	if err != nil {
		return nil, fmt.Errorf("cannot use %q as semver: %s", kernelSemVerStr, err)
	}

	if kernelSemVer.GreaterThan(minSupportedKernelSemVer) || kernelSemVer.Equal(minSupportedKernelSemVer) {
		nodeEligibility.IsEbpfSupported = true
	}

	return nodeEligibility, nil
}

type NodeGroupManagedKernel struct {
	NodeGroupName    string
	HasManagedKernel bool
}

func nodeGroupHasManagedKernel(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ng := &ngv1.NodeGroup{}
	err := sdk.FromUnstructured(obj, ng)
	if err != nil {
		return nil, err
	}

	return &NodeGroupManagedKernel{
		NodeGroupName:    ng.Name,
		HasManagedKernel: pointer.BoolPtrDerefOr(ng.Spec.OperatingSystem.ManageKernel, true),
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: getNodeNameWithSupportedDistro,
		},
		{
			Name:       "nodegroups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: nodeGroupHasManagedKernel,
		},
	},
}, labelNodes)

func labelNodes(input *go_hook.HookInput) error {
	ngSnapshot := input.Snapshots["nodegroups"]

	var ngIsManagedKernelSet = make(map[string]bool, len(ngSnapshot))
	for _, ngIsManagedKernelRaw := range ngSnapshot {
		if ngIsManagedKernelRaw == nil {
			continue
		}
		ngIsManagedKernel := ngIsManagedKernelRaw.(*NodeGroupManagedKernel)
		ngIsManagedKernelSet[ngIsManagedKernel.NodeGroupName] = ngIsManagedKernel.HasManagedKernel
	}

	snapshot := input.Snapshots["nodes"]
	for _, nodeEligibilityRaw := range snapshot {
		if nodeEligibilityRaw == nil {
			continue
		}
		nodeEligibility := nodeEligibilityRaw.(*NodeEligibility)

		input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var node v1.Node
			err := sdk.FromUnstructured(obj, &node)
			if err != nil {
				return nil, err
			}

			// skip nodes with non-managed kernel, can't rely on the correct kernel and kernel headers being present
			if has, ok := ngIsManagedKernelSet[nodeEligibility.NodeGroup]; !ok || !has {
				delete(node.Labels, ebpfSchedulingLabelKey)

				return sdk.ToUnstructured(&node)
			}

			if !nodeEligibility.IsEbpfSupported {
				delete(node.Labels, ebpfSchedulingLabelKey)

				return sdk.ToUnstructured(&node)
			}

			if node.Labels == nil {
				node.Labels = make(map[string]string, 1)
			}
			node.Labels[ebpfSchedulingLabelKey] = ""

			return sdk.ToUnstructured(&node)
		},
			"v1", "Node", "", nodeEligibility.Name)
	}

	return nil
}
