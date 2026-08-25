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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
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

	// registryConfigSnap is the resolved configuration of the registry module: cluster-scoped,
	// singleton, and on a cluster running that module the only description of the registry that is kept
	// current.
	//
	// Preferred over the secret below for the fields that describe WHICH registry this cluster belongs
	// to. The secret is written once, at bootstrap, and then only by this module out of these very
	// values — so on a cluster whose registry has moved since, it says where the registry used to be.
	// Measured on a migrated cluster: the upstream had been moved to
	// `dev-registry.deckhouse.io/sys/deckhouse-oss`, while the legacy contour still named the mirror the
	// cluster came from, with that mirror's robot account.
	registryConfigResourceName = "registry"
)

// registryUpstream is what the resource says about the registry this cluster pulls from. Flat and
// small on purpose: only the fields the global values need.
type registryUpstream struct {
	Address  string `json:"address"`
	Path     string `json:"path"`
	Scheme   string `json:"scheme"`
	CA       string `json:"ca"`
	Username string `json:"username"`
	Password string `json:"password"`
}

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
}, dependency.WithExternalDependencies(discoveryDeckhouseRegistry))

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

// resolvedUpstream reads the upstream out of the registry module's resolved configuration.
//
// Read on demand rather than subscribed to, and that is not a detail. A kubernetes binding on a kind
// the cluster does not have cannot be enabled at all — "Cannot get GroupVersionResource info for
// apiVersion 'deckhouse.io/v1alpha1' kind 'RegistryConfig'" — and a GLOBAL hook that cannot enable its
// bindings takes the whole queue with it. Every cluster without this module installed is such a cluster.
// The same shape cost this platform a wedged operator once already, through another module's hooks
// starting before its own CRD existed.
//
// So: one GET, and every way it can fail to answer is an answer of "ask something else". An absent
// upstream is one of those: a cluster with no upstream at all is what air-gap means.
func resolvedUpstream(ctx context.Context, dc dependency.Container) (*registryUpstream, error) {
	client, err := dc.GetK8sClient()
	if err != nil {
		return nil, fmt.Errorf("getting the kubernetes client: %w", err)
	}

	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "registryconfigs"}

	object, err := client.Dynamic().Resource(gvr).Get(ctx, registryConfigResourceName, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err), meta.IsNoMatchError(err):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("reading RegistryConfig: %w", err)
	}

	upstream, found, err := unstructured.NestedMap(object.Object, "spec", "primary", "upstream")
	if err != nil {
		return nil, fmt.Errorf("read the upstream from RegistryConfig: %w", err)
	}
	if !found {
		return nil, nil
	}

	host, _ := upstream["host"].(string)
	if host == "" {
		return nil, nil
	}

	result := &registryUpstream{Address: host}
	result.Path, _ = upstream["path"].(string)
	result.CA, _ = upstream["ca"].(string)

	// Lower-cased: this resource says `HTTPS`, and the global values accept `http` or `https` only.
	// Measured while repairing a cluster by hand — the other spelling fails the values schema and takes
	// the whole main queue down with it.
	result.Scheme = "https"
	if scheme, _ := upstream["scheme"].(string); scheme != "" {
		result.Scheme = strings.ToLower(scheme)
	}

	if auth, ok := upstream["auth"].(map[string]interface{}); ok {
		result.Username, _ = auth["username"].(string)
		result.Password, _ = auth["password"].(string)

		if result.Username == "" {
			if encoded, _ := auth["auth"].(string); encoded != "" {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					return nil, fmt.Errorf("decode the upstream credentials: %w", err)
				}
				if user, pass, cut := strings.Cut(string(decoded), ":"); cut {
					result.Username, result.Password = user, pass
				}
			}
		}
	}

	return result, nil
}

func discoveryDeckhouseRegistry(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	registryConfSnap, err := sdkobjectpatch.UnmarshalToStruct[registrySecret](input.Snapshots, imageModulesD8RegistryConfSnap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal %s snapshot: %w", imageModulesD8RegistryConfSnap, err)
	}

	// Two sources, and the ORDER is the point of this whole function.
	//
	// The resource is preferred because it is the one kept current: the registry module resolves it from
	// `mc/registry` on every change. The secret is written at bootstrap and afterwards only by this
	// platform out of these very values, so on a cluster whose registry has moved it describes where the
	// registry used to be. Measured on a migrated cluster: the upstream had been moved to
	// `dev-registry.deckhouse.io/sys/deckhouse-oss` while the contour still named the mirror the cluster
	// came from, with that mirror's robot account — and everything downstream believed the contour.
	//
	// And the secret is no longer REQUIRED, which is the other half. Refusing to run without it made
	// this hook the thing that could deadlock a cluster: it runs at Operator-Startup, so a missing
	// secret stopped the main queue before ConvergeModules, which is what would have removed the
	// condition. Measured, on a cluster where that secret was deleted on purpose: nine tasks behind this
	// one and no way out but recreating the secret by hand.
	registrySecretRaw, fromSecret := registrySecret{}, false
	if len(registryConfSnap) > 0 {
		registrySecretRaw, fromSecret = registryConfSnap[0], true
	}

	resolvedFromResource, err := resolvedUpstream(ctx, dc)
	if err != nil {
		return err
	}

	if resolvedFromResource != nil {
		resolved := *resolvedFromResource
		input.Logger.Info("taking the registry from the resource the registry module keeps current",
			slog.String("address", resolved.Address+resolved.Path))

		dockercfg, err := registry_helpers.DockerCfgFromCreds(resolved.Username, resolved.Password, resolved.Address)
		if err != nil {
			return fmt.Errorf("build the docker config from RegistryConfig: %w", err)
		}

		registrySecretRaw = registrySecret{
			RegistryDockercfg: dockercfg,
			Address:           resolved.Address,
			Path:              resolved.Path,
			Scheme:            resolved.Scheme,
			CA:                resolved.CA,
		}
		fromSecret = false
	}

	if !fromSecret && registrySecretRaw.Address == "" {
		// Neither source has anything: a cluster mid-bootstrap, before either object exists.
		return fmt.Errorf("neither the RegistryConfig resource nor the 'deckhouse-registry' secret describes a registry yet")
	}

	if string(registrySecretRaw.RegistryDockercfg) == "" {
		return fmt.Errorf("docker config not found in 'deckhouse-registry' secret")
	}

	if registrySecretRaw.Address == "" {
		return fmt.Errorf("address field not found in 'deckhouse-registry' secret")
	}
	// yes, we store base64 encoded string but in secret object store decoded data
	// In values we store base64-encoded docker config because in this form it is applied in other places.
	registryConfEncoded := base64.StdEncoding.EncodeToString(registrySecretRaw.RegistryDockercfg)

	// `base` is the address container image references are rendered from, and it is the only
	// one of these values that the registry module can move.
	//
	// The others describe the registry the cluster was installed with, and they keep doing
	// that: `address`, `path` and the docker config are what an out-of-cluster caller reads —
	// dhctl among them, which refuses to touch a cluster whose docker config has no
	// credentials for the host they name. Whether anything in the cluster fetches through the
	// node agent instead is decided from `base`, by the party that has to dial it.
	imageBase := fmt.Sprintf("%s%s", registrySecretRaw.Address, registrySecretRaw.Path)
	if published, err := publishedImageAddress(input); err != nil {
		return err
	} else if published != "" {
		input.Logger.Info("rendering image references from the address the registry module published",
			slog.String("address", published))
		imageBase = published
	}

	input.Values.Set("global.modulesImages.registry.base", imageBase)
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
