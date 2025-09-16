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
	"context"
	"encoding/base64"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
)

const (
	imageModulesD8RegistryConfSnap = "d8_registry_secret"
)

type registrySecret struct {
	RegistryDockercfg []byte
	Address           string
	Path              string
	Scheme            string
	CA                string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       imageModulesD8RegistryConfSnap,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-registry"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyD8RegistrySecretFilter,
		},
	},
}, discoveryDeckhouseRegistry)

func applyD8RegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

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

func discoveryDeckhouseRegistry(_ context.Context, input *go_hook.HookInput) error {
	registryConfSnap, err := sdkobjectpatch.UnmarshalToStruct[registrySecret](input.Snapshots, imageModulesD8RegistryConfSnap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", imageModulesD8RegistryConfSnap, err)
	}

	if len(registryConfSnap) == 0 {
		return fmt.Errorf("not found 'deckhouse-registry' secret")
	}

	registrySecretRaw := registryConfSnap[0]

	if string(registrySecretRaw.RegistryDockercfg) == "" {
		return fmt.Errorf("docker config not found in 'deckhouse-registry' secret")
	}

	if registrySecretRaw.Address == "" {
		return fmt.Errorf("address field not found in 'deckhouse-registry' secret")
	}
	// yes, we store base64 encoded string but in secret object store decoded data
	// In values we store base64-encoded docker config because in this form it is applied in other places.
	registryConfEncoded := base64.StdEncoding.EncodeToString(registrySecretRaw.RegistryDockercfg)

	input.Values.Set("global.modulesImages.registry.base", fmt.Sprintf("%s%s", registrySecretRaw.Address, registrySecretRaw.Path))
	input.Values.Set("global.modulesImages.registry.dockercfg", registryConfEncoded)
	input.Values.Set("global.modulesImages.registry.scheme", registrySecretRaw.Scheme)
	input.Values.Set("global.modulesImages.registry.CA", registrySecretRaw.CA)
	input.Values.Set("global.modulesImages.registry.address", registrySecretRaw.Address)
	input.Values.Set("global.modulesImages.registry.path", registrySecretRaw.Path)

	// Create registry config and calculate hash
	registryConfig := deckhouse_registry.Config{
		Address:      registrySecretRaw.Address,
		Path:         registrySecretRaw.Path,
		Scheme:       registrySecretRaw.Scheme,
		CA:           registrySecretRaw.CA,
		DockerConfig: registrySecretRaw.RegistryDockercfg,
	}

	hash, err := registryConfig.Hash()
	if err != nil {
		return fmt.Errorf("failed to calculate registry config hash: %w", err)
	}

	input.Values.Set("global.modulesImages.registry.hash", hash)
	return nil
}
