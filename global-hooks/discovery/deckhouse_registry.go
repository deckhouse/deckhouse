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
	"encoding/json"
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
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_helpers "github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
)

const (
	imageModulesD8RegistryConfSnap = "d8_registry_secret"
	imageAddressSnap               = "registry_image_address"
	inClusterAccessSnap            = "registry_incluster_access"

	// imageAddressConfigMapName is where the registry module publishes the address
	// container image references are to be rendered from.
	//
	// Read here rather than decided here. Whether the cluster's pull path is managed at
	// all, whether the previous implementation of that module is still the one managing
	// it, and whether every node's agent has applied the layout it was given are three
	// questions that module already answers; asking them a second time in a global hook
	// would put two writers on one decision, and the one that got it wrong would point
	// every image reference in the cluster at something nothing can pull.
	imageAddressConfigMapName = registry_const.ImageAddressConfigMapName
	imageAddressConfigMapKey  = registry_const.ImageAddressConfigMapKey

	// registryConfigSnap is the resolved configuration of the registry module: cluster-scoped,
	// singleton, and on a cluster running that module the only description of the registry that is kept
	// current.
	//
	// Preferred over the secret below for the fields that describe WHICH registry this cluster belongs
	// to. The secret is written once, at bootstrap, and then only by this module out of these very
	// values — so on a cluster whose registry has moved since, it says where the registry used to be.
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
		{
			Name:       inClusterAccessSnap,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{registry_const.InClusterAccessSecretName},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyInClusterAccessFilter,
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

// inClusterAccess is the account and authority a pod reads the in-cluster key with, as the registry
// module publishes them once its Service leads a pod to the store.
type inClusterAccess struct {
	Name         string `json:"name"`
	DockerConfig []byte `json:"dockercfg"`
	CA           string `json:"ca"`
}

func applyInClusterAccessFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	return &inClusterAccess{
		Name:         secret.Name,
		DockerConfig: secret.Data["dockercfg"],
		CA:           string(secret.Data["ca"]),
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

	// Lower-cased: this resource says `HTTPS`, and the global values accept `http` or `https` only. The
	// other spelling fails the values schema and takes the whole main queue down with it.
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
	// registry used to be — and everything downstream believes whichever of the two is read first.
	//
	// And the secret is no longer REQUIRED, which is the other half. Refusing to run without it made
	// this hook the thing that could deadlock a cluster: it runs at Operator-Startup, so a missing
	// secret stops the main queue before ConvergeModules — which is what would have removed the
	// condition — leaving no way out but recreating the secret by hand.
	registrySecretRaw, fromSecret := registrySecret{}, false
	if len(registryConfSnap) > 0 {
		registrySecretRaw, fromSecret = registryConfSnap[0], true
	}

	published, err := publishedImageAddress(input)
	if err != nil {
		return err
	}

	// A cluster whose pull path goes through the node agent has one registry as far as anything in it
	// is concerned: the in-cluster one. The upstream, and the account for it, exist in the registry
	// module's configuration and in the layout every agent is given — and nowhere else.
	//
	// So neither the secret nor the resource is read here. The secret is rendered out of these very
	// values, and taking them from the resource's upstream is what put that upstream's account into
	// the secret and into every pull secret copied from it, and what sent every process that reads it
	// — registry-packages-proxy, the Deckhouse controller — straight to the upstream past the agent
	// beside it.
	if published != "" {
		input.Logger.Info("the cluster pulls through the node agent, so it is described as the in-cluster registry",
			slog.String("address", published))
		registry, err := withInClusterAccess(input, inClusterRegistry())
		if err != nil {
			return err
		}
		return setRegistryValues(input, registry, published)
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
	return setRegistryValues(input, registrySecretRaw,
		fmt.Sprintf("%s%s", registrySecretRaw.Address, registrySecretRaw.Path))
}

// inClusterRegistry is the in-cluster registry as the platform records it.
//
// No authority and no account of its own. Whatever dials this address from a node goes through the
// agent, which serves under an authority generated on that node and asks the client for no
// credentials; a container runtime is redirected into the agent by a drop-in, and the agent drops
// whatever a pull secret offers it. The upstream's account, which used to be here, must never be. A pod
// reaches the address through the registry module's Service instead, and the module's account for pods
// (registry-pods: the platform's image set, not the module sources) and the store's authority for that
// are added by withInClusterAccess once the module publishes them.
//
// The host is still in the docker config, without credentials, because what reads the secret looks
// the registry up by host there: the admission webhook refuses a secret with no registry in it, and
// dhctl refuses a cluster whose docker config does not name the host it is told about.
func inClusterRegistry() registrySecret {
	return registrySecret{
		RegistryDockercfg: []byte(fmt.Sprintf(`{"auths":{%q:{}}}`, registry_const.Host)),
		Address:           registry_const.Host,
		Path:              registry_const.Path,
		Scheme:            registry_const.Scheme,
	}
}

// withInClusterAccess adds the account and authority for the key once the registry module publishes them.
//
// A pod has no agent: it dials the key, and the registry module's Service leads it to the store, which
// asks for an account and serves under the module's authority. Without them a module that fetches from a
// pod by these values, such as operator-trivy, fails the handshake and then the authorization. Nothing on a
// node is affected: a container runtime is redirected to the agent, which drops the account, and Deckhouse
// itself drops both for the key. Until they are published the key is described without them, as before.
func withInClusterAccess(input *go_hook.HookInput, registry registrySecret) (registrySecret, error) {
	published, err := sdkobjectpatch.UnmarshalToStruct[inClusterAccess](input.Snapshots, inClusterAccessSnap)
	if err != nil {
		return registry, fmt.Errorf("failed to unmarshal %s snapshot: %w", inClusterAccessSnap, err)
	}
	// By name as well as by the binding's selector, which not every client applies.
	var access *inClusterAccess
	for i := range published {
		if published[i].Name == registry_const.InClusterAccessSecretName {
			access = &published[i]
		}
	}
	if access == nil {
		return registry, nil
	}

	// A docker config without the key's host would leave the platform's secret unreadable by what looks
	// the registry up by host (the admission webhook, dhctl), so a malformed one is not taken.
	var config struct {
		Auths map[string]json.RawMessage `json:"auths"`
	}
	if err := json.Unmarshal(access.DockerConfig, &config); err != nil || config.Auths[registry_const.Host] == nil {
		input.Logger.Warn("the registry module's in-cluster access has no account for the key, describing it without one",
			slog.String("secret", registry_const.InClusterAccessSecretName))
		return registry, nil
	}

	registry.RegistryDockercfg = access.DockerConfig
	registry.CA = access.CA
	return registry, nil
}

// setRegistryValues writes the registry the platform renders from.
//
// `base` is passed separately because it is the one value the registry module moves on its own: every
// other field describes a registry, and `base` is where image references point, which on a cluster
// pulling through the agent is the in-cluster address before the rest of the description follows it.
func setRegistryValues(input *go_hook.HookInput, registrySecretRaw registrySecret, imageBase string) error {
	// yes, we store base64 encoded string but in secret object store decoded data
	// In values we store base64-encoded docker config because in this form it is applied in other places.
	registryConfEncoded := base64.StdEncoding.EncodeToString(registrySecretRaw.RegistryDockercfg)

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
