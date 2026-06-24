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

// Package masterendpoints publishes the Ready control-plane node InternalIPs to
// registry.internal.bootstrapMasterEndpoints. A JOINING node's containerd uses
// these as a pre-CNI registry fallback: https://<master-ip>:5001 reaches the
// master's hostNetwork registry-agent (node-IP reachable before the joining
// node has CNI/CoreDNS/kube-proxy), so the node can pull its own agent + CNI
// images. Once the node's own agent comes up it takes over registry.d (see
// images/registry-agent .../internal/containerd: the .managed-by-agent marker)
// and this bootstrap mirror is dropped.
package masterendpoints

import (
	"context"
	"fmt"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	queue      = "/modules/registry/master-endpoints"
	snapName   = "master-nodes"
	valuesPath = "registry.internal.bootstrapMasterEndpoints"
)

// masterNode is the filtered control-plane node: its InternalIP and Ready state
// (only a Ready master can serve the registry).
type masterNode struct {
	InternalIP string `json:"internalIP"`
	Ready      bool   `json:"ready"`
}

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue:        queue,
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       snapName,
				ApiVersion: "v1",
				Kind:       "Node",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
				},
				FilterFunc: filterMasterNode,
			},
		},
	},
	handle,
)

func filterMasterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, fmt.Errorf("convert node %q: %w", obj.GetName(), err)
	}

	var ip string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ip = addr.Address
			break
		}
	}

	var ready bool
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	return masterNode{InternalIP: ip, Ready: ready}, nil
}

func handle(_ context.Context, input *go_hook.HookInput) error {
	values := helpers.NewValuesAccessor[[]string](input, valuesPath)

	// New arch only: the joining-node bootstrap mirror is a new-arch concept; in
	// legacy/orchestrator mode the node-local registry-proxy handles this path.
	if !helpers.IsNewArchControl(input) {
		values.Clear()
		return nil
	}

	nodes, err := sdkobjectpatch.UnmarshalToStruct[masterNode](input.Snapshots, snapName)
	if err != nil {
		return fmt.Errorf("unmarshal %s snapshot: %w", snapName, err)
	}

	seen := make(map[string]struct{}, len(nodes))
	ips := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if !n.Ready || n.InternalIP == "" {
			continue
		}
		if _, dup := seen[n.InternalIP]; dup {
			continue
		}
		seen[n.InternalIP] = struct{}{}
		ips = append(ips, n.InternalIP)
	}
	sort.Strings(ips)

	values.Set(ips)
	return nil
}
