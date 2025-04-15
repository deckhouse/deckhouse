// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// nodeTarget is a piece of configuration for ping exporter. It represents a single node instance.
type nodeTarget struct {
	Name    string `json:"name"`
	Address string `json:"ipAddress"`
}

type targets struct {
	Cluster []nodeTarget `json:"cluster_targets"`
}

func newTargets(length int) *targets {
	return &targets{
		Cluster: make([]nodeTarget, length),
	}
}

func getAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := &v1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, err
	}

	if node.Spec.Unschedulable {
		return nil, nil
	}
	target := nodeTarget{Name: node.Name}
	for _, address := range node.Status.Addresses {
		if address.Type == v1.NodeInternalIP {
			target.Address = address.Address
			break
		}
	}

	return target, nil
}

func getConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	configMap := &v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, configMap)
	if err != nil {
		return nil, err
	}

	return configMap, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{
			Name: "node_list",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "addresses",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: getAddress,
		},
		{
			Name:       "configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"monitoring-ping-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			FilterFunc: getConfigMap,
		},
	},
}, updateNodeList)

func updateNodeList(input *go_hook.HookInput) error {
	lenSnapshot := len(input.Snapshots["node_list"])
	targets := newTargets(lenSnapshot)

	var configMap *v1.ConfigMap

	if len(input.Snapshots["configmap"]) > 0 {
		configMap = input.Snapshots["configmap"][0].(*v1.ConfigMap)
	}

	for _, item := range input.Snapshots["addresses"] {
		if item == nil {
			continue
		}
		nt := item.(nodeTarget)
		if nt.Address != "" {
			targets.Cluster = append(targets.Cluster, nt)
		}
	}

	if configMap != nil {
		jsonData, err := json.Marshal(targets)
		if err != nil {
			return err
		}

		patch := map[string]interface{}{
			"data": map[string]string{
				"targets.json": string(jsonData),
			},
		}

		input.PatchCollector.PatchWithMerge(
			patch,
			configMap.APIVersion, "ConfigMap", configMap.Namespace, configMap.Name,
		)
	}

	return nil
}
