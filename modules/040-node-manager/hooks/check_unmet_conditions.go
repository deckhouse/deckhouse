// Copyright 2025 Flant JSC
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
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const unmetCloudConditionsKey = "nodeManager:unmetCloudConditions"

type CloudCondition struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Ok      bool   `json:"ok"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "conditions",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-conditions"},
			},
			FilterFunc: updateCloudConditionsFilter,
		},
	},
}, checkCloudConditions)

func checkCloudConditions(input *go_hook.HookInput) error {
	if len(input.Snapshots["conditions"]) == 0 {
		requirements.SaveValue(unmetCloudConditionsKey, false)
		return nil
	}

	conditions := input.Snapshots["conditions"][0].([]CloudCondition)

	if len(conditions) == 0 {
		requirements.SaveValue(unmetCloudConditionsKey, false)
		return nil
	}

	var unmetConditions bool
	for i := range conditions {
		if !conditions[i].Ok {
			unmetConditions = true
			break
		}
	}

	requirements.SaveValue(unmetCloudConditionsKey, unmetConditions)
	return nil
}

func updateCloudConditionsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := new(v1.ConfigMap)
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, err
	}

	var conditions []CloudCondition
	return conditions, json.Unmarshal([]byte(cm.Data["conditions"]), &conditions)
}
