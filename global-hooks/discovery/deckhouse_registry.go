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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
)

const (
	imageModulesD8RegistryConfSnap = "d8_registry_secret"
	imageAddressSnap               = "registry_image_address"

	// imageAddressConfigMapName is where the registry module publishes the address
	// container image references are to be rendered from.
	//
	// Read here rather than decided here. Whether the cluster's pull path is managed at
	// all, whether the previous implementation of that module is still the one managing
	// it, and whether every node's agent has applied the layout it was given are three
	// questions that module already answers; asking them a second time in a global hook
	// would put two writers on one decision, and the one that got it wrong would point
	// every image reference in the cluster at something nothing can pull.
	imageAddressConfigMapName = "registry-image-address"
	imageAddressConfigMapKey  = "base"
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
		{
			Name:       imageAddressSnap,
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{imageAddressConfigMapName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyImageAddressFilter,
		},
	},
}, discoveryDeckhouseRegistry)

func applyD8RegistrySecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
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

func applyImageAddressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap

	if err := sdk.FromUnstructured(obj, &cm); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	return cm.Data[imageAddressConfigMapKey], nil
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

	// `base` is the address container image references are rendered from, and it is the
	// only one of these values that the registry module can move.
	//
	// The others describe the registry the cluster was installed with, and they keep
	// describing it: they are read by the Deckhouse controller's own HTTP client — the
	// release check, the default module source — which has no node agent in its path and
	// cannot reach an in-cluster address. Image references are pulled by the container
	// runtime, which does.
	upstreamBase := fmt.Sprintf("%s%s", registrySecretRaw.Address, registrySecretRaw.Path)
	imageBase, fetchBase := upstreamBase, upstreamBase

	if published, err := publishedImageAddress(input); err != nil {
		return err
	} else if published != "" {
		input.Logger.Info("rendering image references from the address the registry module published",
			slog.String("address", published))
		imageBase = published

		// The Deckhouse controller fetches over HTTP from a process, and a process has to
		// dial. It cannot use the address above: that one names a Service which exists only
		// when there is a cache, and which is never dialled — a container runtime reaches it
		// through a drop-in that redirects any registry to the node agent. What always
		// answers on a node running the agent is the agent itself, on the loopback address.
		//
		// Conditional on the same published address, because it is the same fact: the module
		// publishes only once every node's agent is applying the layout it was given, and
		// until then there is no agent to fetch through either.
		fetchBase = constant.ProxyHostWithPath
	}

	input.Values.Set("global.modulesImages.registry.base", imageBase)
	input.Values.Set("global.modulesImages.registry.fetchBase", fetchBase)
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

// publishedImageAddress is the address the registry module asks image references to be
// rendered from, or empty when it asks for nothing.
//
// Empty is the normal case and the safe one: a cluster whose pull path the module does not
// manage, one still on the module's previous implementation, and one where the node agents
// have not yet applied the layout they were given all leave this unset, and image
// references keep naming the registry the cluster was installed with.
func publishedImageAddress(input *go_hook.HookInput) (string, error) {
	published, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, imageAddressSnap)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal %s snapshot: %w", imageAddressSnap, err)
	}

	if len(published) == 0 {
		return "", nil
	}

	return published[0], nil
}
