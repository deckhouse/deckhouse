/*
Copyright 2025 Flant JSC

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

package bashible

import (
	"errors"
	"fmt"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	masterNodeLabel   = "node-role.kubernetes.io/control-plane"
	versionAnnotation = "registry.deckhouse.io/version"
	secretName        = "registry-bashible-config"

	nodesSnapName  = "nodes"
	secretSnapName = "secret"

	containerdCfgLabel       = "node.deckhouse.io/containerd-config-registry"
	containerdCfgModeCustom  = "custom"
	containerdCfgModeDefault = "default"
)

type hookNodeInfo struct {
	IsMaster          bool
	Version           string
	InternalIP        string
	ContainerdCfgMode string
}

type hookIsSecretExist = bool

func snapName(prefix, name string) string {
	return fmt.Sprintf("%s-->%s", prefix, name)
}

func KubernetesConfig(name string) []go_hook.KubernetesConfig {
	return []go_hook.KubernetesConfig{
		{
			Name:       snapName(name, nodesSnapName),
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var node corev1.Node
				if err := sdk.FromUnstructured(obj, &node); err != nil {
					return nil, fmt.Errorf("failed to convert node %q: %w", obj.GetName(), err)
				}

				info := hookNodeInfo{
					Version:           node.Annotations[versionAnnotation],
					ContainerdCfgMode: node.Labels[containerdCfgLabel],
				}

				if info.Version == "" {
					info.Version = registry_const.UnknownVersion
				}

				_, info.IsMaster = node.Labels[masterNodeLabel]

				for _, addr := range node.Status.Addresses {
					if addr.Type == corev1.NodeInternalIP {
						info.InternalIP = addr.Address
						break
					}
				}

				return helpers.NewKeyValue(node.Name, info), nil
			},
		},
		{
			Name:              snapName(name, secretSnapName),
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: helpers.NamespaceSelector,
			NameSelector: &types.NameSelector{
				MatchNames: []string{secretName},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var secret corev1.Secret
				if err := sdk.FromUnstructured(obj, &secret); err != nil {
					return nil, fmt.Errorf("failed to convert secret %q: %w", obj.GetName(), err)
				}

				return hookIsSecretExist(true), nil
			},
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	nodesInfo, err := helpers.SnapshotToMap[string, hookNodeInfo](input, snapName(name, nodesSnapName))
	if err != nil {
		return Inputs{}, fmt.Errorf("unable to read nodes snapshot: %w", err)
	}

	isSecretExist, err := helpers.SnapshotToSingle[hookIsSecretExist](input, snapName(name, secretSnapName))
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return Inputs{}, fmt.Errorf("unable to read secret snapshot: %w", err)
	}

	inputs := Inputs{
		IsSecretExist: isSecretExist,
		NodeStatus:    make(map[string]InputsNodeStatus),
	}

	for nodeName, nodeInfo := range nodesInfo {
		if nodeInfo.IsMaster && nodeInfo.InternalIP != "" {
			inputs.MasterNodesIPs = append(inputs.MasterNodesIPs, nodeInfo.InternalIP)
		}
		inputs.NodeStatus[nodeName] = InputsNodeStatus{
			Version:           nodeInfo.Version,
			ContainerdCfgMode: nodeInfo.ContainerdCfgMode,
		}
	}

	sort.Strings(inputs.MasterNodesIPs)

	return inputs, nil
}
