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

// Package v2 holds the hooks of the controller-based implementation.
//
// Kept in its own directory, with no shared state with the hooks of the legacy
// implementation, so that removing the old one is a single reviewable commit rather
// than an untangling.
package v2

import (
	"context"
	"fmt"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/v2/pki"
)

const (
	// PKISecretName persists the generated material across restarts of the module.
	//
	// The values are rebuilt from this secret rather than regenerated, because
	// regenerating would hand every node a new certificate authority and break every
	// pull in flight — on a module restart, which happens routinely.
	PKISecretName = "registry-storage-pki"

	// ServiceName is the in-cluster name every image reference is built from.
	ServiceName = "registry.d8-system.svc"

	// BashibleConfigSecretName is what the bashible apiserver reads to build a node's
	// registry context.
	BashibleConfigSecretName = "registry-bashible-config"

	pkiSnapName            = "storage-pki"
	nodesSnapName          = "master-nodes"
	bashibleConfigSnapName = "bashible-config"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       pkiSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{PKISecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: filterPKISecret,
			},
			{
				Name:       nodesSnapName,
				ApiVersion: "v1",
				Kind:       "Node",
				LabelSelector: &v1meta.LabelSelector{
					MatchLabels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
				},
				FilterFunc: filterNodeAddress,
			},
			{
				// Watched only so that withdrawing it can be conditional. Issuing the
				// delete unconditionally would mean a request on every reconciliation of
				// every cluster, for an object that is absent on almost all of them.
				Name:       bashibleConfigSnapName,
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{BashibleConfigSecretName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					// Only its existence matters.
					return obj.GetName(), nil
				},
			},
		},
	},
	handle,
)

func filterPKISecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the secret: %w", err)
	}

	state := secret.Data["state.yaml"]
	if len(state) == 0 {
		return nil, nil
	}
	return state, nil
}

func filterNodeAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("converting the node: %w", err)
	}

	// The internal address is the one a node reaches the storage on its neighbour
	// at, and the one the storage publishes its host port on.
	for _, address := range node.Status.Addresses {
		if address.Type == v1core.NodeInternalIP && address.Address != "" {
			return address.Address, nil
		}
	}
	return nil, nil
}

func handle(_ context.Context, input *go_hook.HookInput) error {
	values := accessor(input)
	current := values.Get()

	state := current.PKI
	if !state.Complete() {
		// Nothing usable in the values, so try the secret before generating: a module
		// restart must not hand every node a new certificate authority.
		restored, err := restoreState(input)
		if err != nil {
			input.Logger.Warn("cannot restore the storage PKI, generating a new one", "error", err.Error())
		}
		state = restored
	}

	addresses := masterAddresses(input)
	hosts := pki.Hosts{
		Service:    ServiceName,
		Additional: addresses,
	}

	switch {
	case !state.Complete():
		input.Logger.Info("generating the storage PKI")
		generated, err := pki.Generate(hosts)
		if err != nil {
			return fmt.Errorf("generating the storage PKI: %w", err)
		}
		state = &generated
	default:
		reissued, err := state.EnsureHosts(hosts)
		if err != nil {
			return fmt.Errorf("checking the storage certificates: %w", err)
		}
		if reissued {
			input.Logger.Info("reissued the storage certificates for a changed set of node addresses",
				"hosts", hosts.Additional)
		}
	}

	current.PKI = state
	current.StorageAddresses = addresses

	// Everything below is what helm and the nodes read, and it is built only when the
	// switch has actually happened. Building it regardless would be harmless in itself,
	// but it would put a node configuration that hands the container runtime to the
	// agent into the values of a cluster the legacy implementation still owns — one
	// template gate away from two writers on every node.
	switch {
	case !current.Enabled:
		// The previous implementation still owns the cluster. Cleared rather than left
		// stale: these values are what a node is told, and a leftover would keep telling
		// nodes the agent owns their runtime configuration.
		current.RegistryConfig = nil
		current.BashibleConfig = nil

	default:
		parsed, err := readSettings(input)
		if err != nil {
			return err
		}

		config := buildRegistryConfig(parsed)
		current.RegistryConfig = &config

		if config.Mode == string(registryv1alpha1.ModeUnmanaged) {
			// Unmanaged means nothing, all the way down to the node. No agent, no node
			// configuration of ours at all — the bashible apiserver then falls back to
			// the `deckhouse-registry` secret and points the container runtime straight
			// at the registry the cluster was installed with, which is the behaviour of
			// a cluster where this module was never enabled.
			//
			// This is also the state a migrating cluster lands in the moment the previous
			// implementation lets go, so the fallback has to be the thing that carries it
			// — see withdrawNodeConfiguration.
			current.BashibleConfig = nil
			withdrawNodeConfiguration(input)
			break
		}

		nodeConfig, err := buildBashibleConfig(config, registryv1alpha1.Auth{
			Username: state.RO.Name,
			Password: state.RO.Password,
		}, state.CA.Cert, addresses)
		if err != nil {
			return err
		}
		current.BashibleConfig = nodeConfig
	}

	values.Set(current)
	return nil
}

// withdrawNodeConfiguration removes the node configuration secret.
//
// Helm cannot: the secret carries `helm.sh/resource-policy: keep`, because losing it
// mid-transition would reconfigure every node in the cluster at once. So while the module
// manages nothing, it is removed explicitly — and it has to be removed rather than left
// behind, since the bashible apiserver prefers it over its fallback and a cluster that
// just finished migrating would otherwise keep following the configuration the previous
// implementation left.
func withdrawNodeConfiguration(input *go_hook.HookInput) {
	if _, err := helpers.SnapshotToSingle[string](input, bashibleConfigSnapName); err != nil {
		return
	}

	input.Logger.Info("the module manages nothing, withdrawing the node registry configuration",
		"secret", BashibleConfigSecretName)
	input.PatchCollector.Delete("v1", "Secret", "d8-system", BashibleConfigSecretName)
}

func restoreState(input *go_hook.HookInput) (*pki.State, error) {
	raw, err := helpers.SnapshotToSingle[[]byte](input, pkiSnapName)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var state pki.State
	if err := yaml.Unmarshal(raw, &state); err != nil {
		return nil, fmt.Errorf("decoding the stored PKI: %w", err)
	}
	return &state, nil
}

// masterAddresses returns the node addresses the storage is reached at, sorted so
// that the API returning them in a different order does not look like a change and
// reissue the certificates.
func masterAddresses(input *go_hook.HookInput) []string {
	addresses, err := helpers.SnapshotToList[string](input, nodesSnapName)
	if err != nil {
		input.Logger.Warn("cannot read the master node addresses", "error", err.Error())
		return nil
	}

	slices.Sort(addresses)
	return slices.Compact(addresses)
}
