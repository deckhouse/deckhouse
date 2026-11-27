/*
Copyright 2026 Flant JSC

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

package v2

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	effectiveUpstreamSnapName = "effective-upstream"
	upstreamAuthSnapName      = "upstream-auth"
)

// The proxy for pods without the cache (templates/v2/storage/proxy.yaml) fetches from the upstream the
// nodes fetch from: the one the registry controller holds, not the configured one. A configured upstream
// that fails its probe is not used by the nodes, which stay on the last working one; the proxy follows,
// and before any has passed, it follows them onto the configured one as well.
var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 17},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       effectiveUpstreamSnapName,
				ApiVersion: "deckhouse.io/v1alpha1",
				Kind:       "RegistryConfig",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"registry"},
				},
				FilterFunc: filterEffectiveUpstream,
			},
			{
				// The credentials of the upstream in effect, under AuthKeyUpstream: the controller keeps
				// them with the upstream it holds.
				Name:       upstreamAuthSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{registry_const.AuthSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterUpstreamAuth,
			},
		},
	},
	handleProxyUpstream,
)

type namedUpstream struct {
	Name     string         `json:"name"`
	Upstream *ProxyUpstream `json:"upstream,omitempty"`
}

func filterEffectiveUpstream(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var config registryv1alpha1.RegistryConfig
	if err := sdk.FromUnstructured(obj, &config); err != nil {
		return nil, fmt.Errorf("converting the registry config: %w", err)
	}

	result := namedUpstream{Name: config.Name}
	upstream := config.Status.EffectiveUpstream
	// None in effect yet: the controller puts the configured one on the nodes until one passes its probe
	// (NothingApplied in its layout), so the proxy follows the nodes there too. Only without the cache,
	// which is the only case the proxy runs in.
	if (upstream == nil || upstream.Host == "") && !config.Spec.Storage.Cache {
		upstream = config.Spec.Primary.Upstream
	}
	if upstream != nil && upstream.Host != "" {
		result.Upstream = &ProxyUpstream{
			Host:   upstream.Host,
			Path:   upstream.Path,
			Scheme: strings.ToLower(string(upstream.Scheme)),
			CA:     upstream.CA,
		}
	}
	return result, nil
}

type namedAuth struct {
	Name string `json:"name"`
	Auth string `json:"auth"`
}

func filterUpstreamAuth(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the resolved auth: %w", err)
	}
	return namedAuth{Name: secret.Name, Auth: string(secret.Data[registry_const.AuthKeyUpstream])}, nil
}

func handleProxyUpstream(_ context.Context, input *go_hook.HookInput) error {
	upstream, err := readProxyUpstream(input)
	if err != nil {
		return err
	}

	values := accessor(input)
	current := values.Get()
	current.ProxyUpstream = upstream
	values.Set(current)
	return nil
}

// readProxyUpstream is the upstream the nodes fetch from with its credentials, nil while there is none.
func readProxyUpstream(input *go_hook.HookInput) (*ProxyUpstream, error) {
	configs, err := helpers.SnapshotToList[namedUpstream](input, effectiveUpstreamSnapName)
	if err != nil {
		return nil, fmt.Errorf("reading the registry config: %w", err)
	}
	auths, err := helpers.SnapshotToList[namedAuth](input, upstreamAuthSnapName)
	if err != nil {
		return nil, fmt.Errorf("reading the resolved auth: %w", err)
	}

	var upstream *ProxyUpstream
	for _, config := range configs {
		if config.Name == "registry" && config.Upstream != nil {
			upstream = config.Upstream
		}
	}
	if upstream == nil {
		return nil, nil
	}
	for _, auth := range auths {
		if auth.Name == registry_const.AuthSecretName {
			upstream.Auth = auth.Auth
		}
	}
	return upstream, nil
}
