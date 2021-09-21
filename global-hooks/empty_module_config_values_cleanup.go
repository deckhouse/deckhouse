// Copyright 2021 Flant JSC
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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "d8cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyDeckhouseEmptyModsFilter,
		},
	},
}, cleanEmptyModulesFromD8Config)

func applyDeckhouseEmptyModsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	// small optimization
	// find empty configurations in filter. then:
	//   we do not need to call hook in every cm edition
	//   optimize check sum calculation
	//   in hook we use PatchCollector.Filter at least one call to api for getting object
	keysToRemove := make([]string, 0)
	for k, v := range cm.Data {
		if v == "{}\n" {
			keysToRemove = append(keysToRemove, k)
		}
	}

	return keysToRemove, nil
}

// cleanEmptyModulesFromD8Config
// removes empty modules configuration
// from deckhouse CM
func cleanEmptyModulesFromD8Config(input *go_hook.HookInput) error {
	d8CmSnap := input.Snapshots["d8cm"]
	if len(d8CmSnap) < 1 {
		input.LogEntry.Warnln("Deckhouse config is not found in snapshots")
		return nil
	}

	keysToRemove := d8CmSnap[0].([]string)
	if len(keysToRemove) == 0 {
		return nil
	}

	removeEmptyModConfigs := func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
		var cm v1core.ConfigMap
		err := sdk.FromUnstructured(u, &cm)
		if err != nil {
			return nil, err
		}

		for _, k := range keysToRemove {
			delete(cm.Data, k)
		}

		return sdk.ToUnstructured(&cm)
	}

	// Filter guarantees
	input.PatchCollector.Filter(removeEmptyModConfigs, "v1", "ConfigMap", "d8-system", "deckhouse")

	return nil
}
