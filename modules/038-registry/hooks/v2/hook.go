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

	// PKIStateSecretName is where that state actually lives, and it is a secret nothing mounts.
	//
	// Separate from the one above because the storage pod mounts that one, and the state is the
	// whole generated material: the certificate authority's private key, the token signing key the
	// registry is deliberately not given, the publication password and the ingress client key. As a
	// key of the mounted secret it was readable at `/pki/state.yaml` by all three containers, which
	// undid every split the render is built on — the authority's key alone issues a certificate
	// every node agent in the cluster trusts.
	PKIStateSecretName = "registry-storage-pki-state"

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
					// Both, because the state moved and a cluster running the previous render
					// still has it only in the mounted secret — see preferredPKIState.
					MatchNames: []string{PKIStateSecretName, PKISecretName},
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

// pkiStateSnapshot is one secret that may still carry the persisted state, named so the reader can
// tell which of the two it came from.
type pkiStateSnapshot struct {
	Secret string `json:"secret"`
	State  []byte `json:"state,omitempty"`
}

func filterPKISecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret
	if err := sdk.FromUnstructured(obj, &secret); err != nil {
		return nil, fmt.Errorf("converting the secret: %w", err)
	}

	state := secret.Data["state.yaml"]
	if len(state) == 0 {
		return nil, nil
	}
	return pkiStateSnapshot{Secret: obj.GetName(), State: state}, nil
}

// preferredPKIState picks which persisted state to restore from.
//
// The state secret first, and the mounted secret only if that is the only place it is left. The
// fallback exists for one transition: a cluster running the previous render has the state nowhere
// else, and the new secret cannot appear before the hook has put the state into the values it is
// rendered from. Reading only the new name there means generating a fresh authority — every node
// agent in the cluster then trusts a certificate the storage no longer presents, and no pull
// succeeds until every layout has been rewritten and applied. It can be dropped once no cluster
// still runs a render that wrote the state into the mounted secret.
func preferredPKIState(found []pkiStateSnapshot) []byte {
	var legacy []byte
	for _, candidate := range found {
		if len(candidate.State) == 0 {
			continue
		}
		if candidate.Secret == PKIStateSecretName {
			return candidate.State
		}
		legacy = candidate.State
	}
	return legacy
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

		if config.Mode == string(registryv1alpha1.ModeUnmanaged) {
			if drain := current.Drain; drain != nil && drain.Active && drain.Config != nil {
				// Asked to leave, but the cluster has not finished moving off the address
				// this module serves: rendered manifests all over it still name
				// `registry.d8-system.svc:5001`, and they move only as the operator
				// re-renders one release at a time. So serving continues from the
				// configuration that was in effect when the user asked, with its mode
				// already set to `Managed` by the drain — every gate below and every
				// template reads that one value, so nothing else here has to know.
				//
				// What the immediate withdrawal cost, measured: 84s during which the
				// platform's own Deployment named a registry that no longer existed, and
				// 680s during which workloads could not pull, 16 at the peak. See
				// `hooks/v2/drain.go`.
				config = *drain.Config
			} else {
				// Unmanaged means nothing, all the way down to the node. No agent, no node
				// configuration of ours at all — the bashible apiserver then falls back to
				// the `deckhouse-registry` secret and points the container runtime straight
				// at the registry the cluster was installed with, which is the behaviour of
				// a cluster where this module was never enabled.
				//
				// This is also the state a migrating cluster lands in the moment the previous
				// implementation lets go, so the fallback has to be the thing that carries it
				// — see withdrawNodeConfiguration.
				current.RegistryConfig = &config
				current.BashibleConfig = nil
				withdrawNodeConfiguration(input)
				break
			}
		}

		current.RegistryConfig = &config

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
	found, err := helpers.SnapshotToList[pkiStateSnapshot](input, pkiSnapName)
	if err != nil {
		return nil, err
	}

	raw := preferredPKIState(found)
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
