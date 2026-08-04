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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

// ImageAddressConfigMapName is where the address container image references are rendered
// from is published for the platform to read.
//
// The address is the module's answer to a question asked outside of it: a global hook
// renders every embedded module's image references, and until this existed it took the
// address from the `deckhouse-registry` secret — the contour this implementation is meant
// to remove. Published as a cluster object rather than a value because the reader is a
// global hook, which runs before any module and cannot see module values. A ConfigMap and
// not a Secret: an address is not a credential, and the credentials for this registry are
// never in an image reference — the node agent supplies them.
//
// Owned by the Helm release, which is what withdraws it. Setting `mode: Unmanaged` or
// disabling the module removes it, and the platform falls back to the registry it was
// installed with. That fallback is the reason this is conditional at all; in the state
// this module is heading for, the address is simply a constant.
const ImageAddressConfigMapName = "registry-image-address"

// ImageAddressKey is the only key in it.
const ImageAddressKey = "base"

const (
	imageAddressSnapName  = "image-address"
	registryNodesSnapName = "registry-nodes"
	allNodesSnapName      = "all-nodes"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		// After the hook that resolves the configuration, whose `manages` answer decides
		// whether the ConfigMap is rendered at all.
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
		Queue:        "/modules/registry/v2",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				// Watched for its own existence, which is what makes the decision
				// below stick. See `switched`.
				Name:       imageAddressSnapName,
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{ImageAddressConfigMapName},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}},
				},
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					return obj.GetName(), nil
				},
			},
			{
				Name:       registryNodesSnapName,
				ApiVersion: registryv1alpha1.GroupVersion.String(),
				Kind:       "RegistryNode",
				FilterFunc: filterRegistryNodeConvergence,
			},
			{
				// Every node, not only the masters: the address is rendered once for the
				// whole cluster, so it is safe only when every node can resolve it.
				Name:       allNodesSnapName,
				ApiVersion: "v1",
				Kind:       "Node",
				FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
					return obj.GetName(), nil
				},
			},
		},
	},
	handleImageAddress,
)

// nodeConvergence is what a single node contributes to the decision.
type nodeConvergence struct {
	// Applied reports that the agent applied the generation it was given, rather than
	// merely reporting success on something else. An agent that cannot use what the API
	// offers serves the layout it was installed with and still reports success, so
	// `Reconciled` alone does not mean the cluster's configuration is in effect.
	Applied bool `json:"applied"`
}

func filterRegistryNodeConvergence(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node registryv1alpha1.RegistryNode
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("converting the registry node: %w", err)
	}

	return nodeConvergence{
		Applied: node.Status.Reconciled &&
			node.Status.ObservedGeneration == node.Generation,
	}, nil
}

func handleImageAddress(_ context.Context, input *go_hook.HookInput) error {
	published, err := readImageAddressState(input)
	if err != nil {
		return err
	}

	values := accessor(input)
	current := values.Get()

	if published.switched() {
		current.ImageAddress = registry_const.HostWithPath
	} else {
		current.ImageAddress = ""
		input.Logger.Info(
			"image references still point at the registry the cluster was installed with",
			"reason", published.blockedReason())
	}

	values.Set(current)
	return nil
}

// imageAddressState is what the decision is made from.
type imageAddressState struct {
	// AlreadyPublished reports that the ConfigMap exists.
	AlreadyPublished bool

	// Nodes is the number of nodes in the cluster.
	Nodes int

	// Layouts is the number of nodes the controller has written a layout for.
	Layouts int

	// Applied is how many of those layouts their agent has applied.
	Applied int
}

// switched answers whether image references may name the in-cluster registry.
//
// The address is rendered once for the whole cluster, and the name it uses resolves
// nowhere except through the node agent — no DNS record answers it, and no pull secret
// carries credentials for it. So it is safe exactly when every node's agent is applying
// the layout the cluster gave it, and switching to it earlier would re-render every
// workload in every module into an image reference nothing can pull yet. That is not a
// theoretical window: on `Unmanaged` -> `Managed` the node configuration reaches nodes
// through a bashible rollout that takes minutes, and a DaemonSet rolled inside it loses
// its pod on every node until the rollout lands.
//
// Sticky, and that is the more important half. Once the cluster is rendering the
// in-cluster address, a single node whose agent falls behind must not take the address
// away again: that would re-render every workload in the cluster back onto the upstream —
// which, in an air-gapped cluster, is an address that answers nothing at all. So the
// question is asked once, and the ConfigMap's own existence is the record that it has been
// answered. It is the Helm release that withdraws it, when the module stops managing the
// pull path or is disabled, and only then is the question asked again.
func (s imageAddressState) switched() bool {
	if s.AlreadyPublished {
		return true
	}

	return s.Nodes > 0 && s.Layouts == s.Nodes && s.Applied == s.Layouts
}

// blockedReason says why in the words of what is missing.
func (s imageAddressState) blockedReason() string {
	switch {
	case s.Nodes == 0:
		return "no nodes are known yet"
	case s.Layouts != s.Nodes:
		return fmt.Sprintf("the controller has written %d layouts for %d nodes", s.Layouts, s.Nodes)
	default:
		return fmt.Sprintf("%d of %d nodes have applied the layout they were given",
			s.Applied, s.Layouts)
	}
}

func readImageAddressState(input *go_hook.HookInput) (imageAddressState, error) {
	result := imageAddressState{}

	if _, err := helpers.SnapshotToSingle[string](input, imageAddressSnapName); err == nil {
		result.AlreadyPublished = true
		return result, nil
	}

	nodes, err := helpers.SnapshotToList[string](input, allNodesSnapName)
	if err != nil {
		return result, fmt.Errorf("reading the nodes: %w", err)
	}
	result.Nodes = len(nodes)

	layouts, err := helpers.SnapshotToList[nodeConvergence](input, registryNodesSnapName)
	if err != nil {
		return result, fmt.Errorf("reading the node layouts: %w", err)
	}
	result.Layouts = len(layouts)

	for _, layout := range layouts {
		if layout.Applied {
			result.Applied++
		}
	}

	return result, nil
}
