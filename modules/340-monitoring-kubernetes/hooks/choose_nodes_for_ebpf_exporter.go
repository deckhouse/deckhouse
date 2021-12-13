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

	"github.com/blang/semver"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	ebpfSchedulingLabelKey = "monitoring-kubernetes.deckhouse.io/ebpf-supported"
)

type NodeEligibility struct {
	Name            string
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

	nodeEligibility := &NodeEligibility{Name: node.Name}

	matches := kernelRegex.FindStringSubmatch(node.Status.NodeInfo.KernelVersion)
	if len(matches) != 2 {
		return nil, fmt.Errorf("failed to match kernel semver in %q with regex %q",
			node.Status.NodeInfo.KernelVersion, kernelRegex.String())
	}
	kernelSemVerStr := matches[1]

	kernelSemVer, err := semver.New(kernelSemVerStr)
	if err != nil {
		return nil, fmt.Errorf("cannot use %q as semver: %s", kernelSemVerStr, err)
	}

	if kernelSemVer.GE(minSupportedKernelSemVer) {
		nodeEligibility.IsEbpfSupported = true
	}

	return nodeEligibility, nil
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
	},
}, labelNodes)

func labelNodes(input *go_hook.HookInput) error {
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
