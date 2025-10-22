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

package bootstrap

import (
	"errors"
	"fmt"
	registry_bootstrap "github.com/deckhouse/deckhouse/go_lib/registry/models/bootstrap"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	SecretName      = "registry-bootstrap"
	SecretNamespace = "d8-system"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:       name,
		ApiVersion: "v1",
		Kind:       "Secret",
		NameSelector: &types.NameSelector{
			MatchNames: []string{SecretName},
		},
		NamespaceSelector: &types.NamespaceSelector{
			NameSelector: &types.NameSelector{
				MatchNames: []string{SecretNamespace},
			},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var secret v1core.Secret

			err := sdk.FromUnstructured(obj, &secret)
			if err != nil {
				return nil, fmt.Errorf("failed to convert secret to struct: %v", err)
			}

			config, ok := secret.Data["config"]
			if !ok {
				return nil, nil
			}

			return config, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	var err error
	ret := Inputs{}

	ret.Config, err = helpers.SnapshotToSingle[registry_bootstrap.Config](input, name)
	if err == nil {
		ret.IsActive = true
	} else {
		if !errors.Is(err, helpers.ErrNoSnapshot) {
			return ret, fmt.Errorf("get RegistryBootstrap snapshot error: %w", err)
		}
	}

	return ret, nil
}

func SecretIsExist(input *go_hook.HookInput, name string) bool {
	return len(input.Snapshots.Get(name)) > 0
}
