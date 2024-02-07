// Copyright 2023 Flant JSC
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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"errors"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	hasNodesWithDocker          = "nodeManager:hasNodesWithDocker"
	containerUnknownVersion     = "unknownVersion"
	nodeSnapName                = "check_nodes_cri"
	notManagedCriMaxKubeVersion = "1.24.0"
	nodeGroupSnapName           = "node_group"
	criTypeNotManaged           = "NotManaged"
	criTypeDocker               = "Docker"
	criTypeContainerd           = "Containerd"
)

type nodeGroupCRIType struct {
	Name    string
	CRIType string
}

type nodeCRIVersion struct {
	NodeGroup               string
	ContainerRuntimeVersion string
	KubeletVersion          string
}

var isDockerRegexp = regexp.MustCompile(`^docker.*?`)

// TODO: Remove this hook after 1.47.1 release
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         nodeSnapName,
			WaitForSynchronization:       pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(true),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "node.deckhouse.io/group",
						Operator: v1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: applyNodesCRIVersionFilter,
		},
		{
			Name:                   nodeGroupSnapName,
			Kind:                   "NodeGroup",
			ApiVersion:             "deckhouse.io/v1",
			WaitForSynchronization: pointer.Bool(false),
			FilterFunc:             applyNodeGroupCRITypeFilter,
		},
	},
}, discoverNodesCRIVersion)

func applyNodesCRIVersionFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	nodeGroup := node.Labels["node.deckhouse.io/group"]
	containerRuntimeVersion := node.Status.NodeInfo.ContainerRuntimeVersion
	kubeletVersion := node.Status.NodeInfo.KubeletVersion

	if containerRuntimeVersion == "" {
		containerRuntimeVersion = containerUnknownVersion
	}

	return nodeCRIVersion{
		NodeGroup:               nodeGroup,
		ContainerRuntimeVersion: containerRuntimeVersion,
		KubeletVersion:          kubeletVersion,
	}, nil
}

func applyNodeGroupCRITypeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return nodeGroupCRIType{
		Name:    ng.GetName(),
		CRIType: ng.Spec.CRI.Type,
	}, nil
}

func discoverNodesCRIVersion(input *go_hook.HookInput) error {
	defaultCRIValue, ok := input.Values.GetOk("global.clusterConfiguration.defaultCRI")
	if !ok {
		return errors.New("defaultCRI absent in clusterConfiguration")
	}
	defaultCRI := defaultCRIValue.String()

	if defaultCRI == criTypeDocker {
		requirements.SaveValue(hasNodesWithDocker, true)
		return nil
	}

	ngSnap := input.Snapshots[nodeGroupSnapName]
	ngCRITypeMap := make(map[string]string)

	notManagedCriKubeVersion, err := semver.NewVersion(notManagedCriMaxKubeVersion)
	if err != nil {
		return err
	}

	for _, item := range ngSnap {
		ng := item.(nodeGroupCRIType)
		if ng.CRIType == criTypeDocker {
			requirements.SaveValue(hasNodesWithDocker, true)
			return nil
		}
		ngCRITypeMap[ng.Name] = ng.CRIType
	}

	nSnap := input.Snapshots[nodeSnapName]
	if len(nSnap) == 0 {
		return nil
	}

	for _, item := range nSnap {
		n := item.(nodeCRIVersion)
		criType, ok := ngCRITypeMap[n.NodeGroup]
		if !ok {
			criType = defaultCRI
		}

		kubeVersion, err := semver.NewVersion(n.KubeletVersion)
		if err != nil {
			return err
		}

		if !isDockerRegexp.MatchString(n.ContainerRuntimeVersion) {
			continue
		}

		// skip if NodeGroup CRI Type == NotManaged and node kubernetes version < notManagedCriKubeVersion
		if criType == criTypeNotManaged &&
			kubeVersion.LessThan(notManagedCriKubeVersion) {
			continue
		}

		requirements.SaveValue(hasNodesWithDocker, true)
		return nil
	}

	requirements.SaveValue(hasNodesWithDocker, false)
	return nil
}
