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
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	hasNodesOtherThanContainerd = "nodeManager:hasNodesOtherThanContainerd"
	containerUnknownVersion     = "unknownVersion"
	nodeSnapName                = "check_nodes_cri"
	notManagedCriMaxKubeVersion = "1.24.0"
	nodeGroupSnapName           = "node_group"
	defaultCRISnapName          = "default_cri"
	criTypeNotManaged           = "NotManaged"
	criTypeDocker               = "Docker"
	criTypeContainerd           = "Containerd"
)

type nodeGroupCRIType struct {
	Name    string
	CriType string
}

type nodeCRIVersion struct {
	NodeGroup               string
	ContainerRuntimeVersion string
	KubeletVersion          string
}

var isContainerdRegexp = regexp.MustCompile(`^containerd.*?`)

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
		{
			Name:                   defaultCRISnapName,
			WaitForSynchronization: pointer.Bool(false),
			ApiVersion:             "v1",
			Kind:                   "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			FilterFunc: applyDefaultCRISecretFilter,
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
		CriType: ng.Spec.CRI.Type,
	}, nil
}

func applyDefaultCRISecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sec corev1.Secret

	err := sdk.FromUnstructured(obj, &sec)
	if err != nil {
		return "", err
	}
	clusterConfig, ok := sec.Data["cluster-configuration.yaml"]
	if !ok {
		return "", nil
	}

	var parsedClusterConfig map[string]interface{}
	if err := yaml.Unmarshal(clusterConfig, &parsedClusterConfig); err != nil {
		return "", fmt.Errorf("cannot parse cluster configuration: %v", err)
	}

	if defaultCRI, ok := parsedClusterConfig["defaultCRI"]; ok {
		return defaultCRI, nil
	}
	return "", nil
}

func discoverNodesCRIVersion(input *go_hook.HookInput) error {
	defaultCRISnap := input.Snapshots[defaultCRISnapName]
	defaultCRI := criTypeContainerd

	if len(defaultCRISnap) > 0 {
		defaultCRI = defaultCRISnap[0].(string)
	}

	if defaultCRI == criTypeDocker {
		requirements.SaveValue(hasNodesOtherThanContainerd, true)
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
		ngCRITypeMap[ng.Name] = ng.CriType
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

		if isContainerdRegexp.MatchString(n.ContainerRuntimeVersion) {
			continue
		}

		// skip if NodeGroup CRI Type == NotManaged and node kubernetes version < notManagedCriKubeVersion
		if criType == criTypeNotManaged &&
			kubeVersion.LessThan(notManagedCriKubeVersion) {
			continue
		}

		requirements.SaveValue(hasNodesOtherThanContainerd, true)
		return nil
	}

	requirements.SaveValue(hasNodesOtherThanContainerd, false)
	return nil
}
