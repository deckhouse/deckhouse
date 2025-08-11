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

package pki

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

func KubernetsConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "v1",
		Kind:              "Secret",
		NamespaceSelector: helpers.NamespaceSelector,
		NameSelector: &types.NameSelector{
			MatchNames: []string{
				"registry-pki",
			},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var secret v1core.Secret

			err := sdk.FromUnstructured(obj, &secret)
			if err != nil {
				return nil, fmt.Errorf("failed to convert secret \"%v\" to struct: %v", obj.GetName(), err)
			}

			ret := State{
				CA:    secretDataToCertModel(secret, "ca"),
				Token: secretDataToCertModel(secret, "token"),
			}

			return ret, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	return helpers.SnapshotToSingle[Inputs](input, name)
}
