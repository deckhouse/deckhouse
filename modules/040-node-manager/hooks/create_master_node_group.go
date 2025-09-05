// Copyright 2022 Flant JSC
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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// ensure crds hook has order 5, for creating node group we should use greater number
	OnStartup: &go_hook.OrderedConfig{Order: 6},
}, createMasterNodeGroup)

func getDefaultMasterNg(clusterType string) (*unstructured.Unstructured, error) {
	// do not use internal type because internal type has none pointer struct fields
	// this fields not skip due marshaling and validation will be fail on fields
	// CRI and CloudInstances
	// CloudInstances is incorrect field for master nodes
	spec := map[string]interface{}{
		"nodeType": "CloudPermanent",
		"disruptions": map[string]interface{}{
			"approvalMode": "Manual",
		},
		"nodeTemplate": map[string]interface{}{
			"labels": map[string]interface{}{
				"node-role.kubernetes.io/control-plane": "",
				"node-role.kubernetes.io/master":        "", // preserve legacy node role for backward compatibility with user software
			},
			"taints": []map[string]interface{}{
				{
					"key":    "node-role.kubernetes.io/control-plane",
					"effect": "NoSchedule",
				},
			},
		},
	}
	if clusterType == "Static" {
		spec["nodeType"] = "Static"
	}

	ng := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": "master",
		},
		"spec": spec,
	}
	o, err := sdk.ToUnstructured(&ng)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func createMasterNodeGroup(_ context.Context, input *go_hook.HookInput) error {
	clusterType := input.Values.Get("global.clusterConfiguration.clusterType").String()

	ng, err := getDefaultMasterNg(clusterType)
	if err != nil {
		return err
	}

	// Do not patch node group if it already exists to avoid conflicts with user changes.
	input.PatchCollector.CreateIfNotExists(ng)

	return nil
}
