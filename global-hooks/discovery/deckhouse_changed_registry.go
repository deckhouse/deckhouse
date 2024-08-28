// Copyright 2024 Flant JSC
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
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	imageModulesD8RegistryChangeConfSnap = "d8_registry_secret_changed"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       imageModulesD8RegistryChangeConfSnap,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry-changed"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyD8RegistryChangedSecretFilter,
		},
	},
}, discoveryDeckhouseRegistryChanged)

func applyD8RegistryChangedSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	var scheme []byte
	scheme, ok := secret.Data["scheme"]
	if !ok {
		scheme = []byte("https")
	}

	return &registrySecret{
		RegistryDockercfg: secret.Data[".dockerconfigjson"],
		Address:           string(secret.Data["address"]),
		Path:              string(secret.Data["path"]),
		Scheme:            string(scheme),
		CA:                string(secret.Data["ca"]),
	}, nil
}

func discoveryDeckhouseRegistryChanged(input *go_hook.HookInput) error {
	registryConfSnap := input.Snapshots[imageModulesD8RegistryChangeConfSnap]

	if len(registryConfSnap) == 0 {
		input.Values.Remove("global.modulesImages.changedRegistry")
		return nil
	}

	registrySecretRaw := registryConfSnap[0].(*registrySecret)

	if string(registrySecretRaw.RegistryDockercfg) == "" {
		return fmt.Errorf("docker config not found in 'deckhouse-registry-changed' secret")
	}

	if registrySecretRaw.Address == "" {
		return fmt.Errorf("address field not found in 'deckhouse-registry-changed' secret")
	}

	registryConfEncoded := base64.StdEncoding.EncodeToString(registrySecretRaw.RegistryDockercfg)

	input.Values.Set("global.modulesImages.changedRegistry.base", fmt.Sprintf("%s%s", registrySecretRaw.Address, registrySecretRaw.Path))
	input.Values.Set("global.modulesImages.changedRegistry.dockercfg", registryConfEncoded)
	input.Values.Set("global.modulesImages.changedRegistry.scheme", registrySecretRaw.Scheme)
	input.Values.Set("global.modulesImages.changedRegistry.CA", registrySecretRaw.CA)
	input.Values.Set("global.modulesImages.changedRegistry.address", registrySecretRaw.Address)
	input.Values.Set("global.modulesImages.changedRegistry.path", registrySecretRaw.Path)
	return nil
}
