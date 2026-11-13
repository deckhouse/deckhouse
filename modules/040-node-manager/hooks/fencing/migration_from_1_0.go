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

package fencing

import (
	"context"
	"fmt"
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	fencing10NodeLabel = "node-manager.deckhouse.io/fencing-enabled"

	fencing10NodesSnapshot = "nodes"
	fencing10PodsSnapshot  = "fencing10_agent_pods"
)

func nameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

// fencing10Node has only the fields that the flag needs, so other changes of a Node
// do not start the hook.
type fencing10Node struct {
	Name  string
	Ready bool
	Label bool
}

func fencing10NodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	if err := sdk.FromUnstructured(obj, &node); err != nil {
		return nil, err
	}

	_, label := node.Labels[fencing10NodeLabel]
	ready := slices.ContainsFunc(node.Status.Conditions, func(c corev1.NodeCondition) bool {
		return c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue
	})

	return fencing10Node{Name: node.Name, Ready: ready, Label: label}, nil
}

func podNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeName, _, err := unstructured.NestedString(obj.Object, "spec", "nodeName")

	return nodeName, err
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		// All Nodes, because a pod of version 1 counts only on a Ready Node.
		{
			Name:       fencing10NodesSnapshot,
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: fencing10NodeFilter,
		},
		// A pod without the version label is an agent 1.0.
		{
			Name:              fencing10PodsSnapshot,
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{agentNamespace}}},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"app": "fencing-agent"},
				MatchExpressions: []metav1.LabelSelectorRequirement{{Key: agentVersionLabel, Operator: metav1.LabelSelectorOpDoesNotExist}},
			},
			FilterFunc: podNodeFilter,
		},
	},
}, handleMigrationFrom10)

// handleMigrationFrom10 is the only writer of nodeManager.internal.fencing.migrationFrom10.
// The flag stays while a Ready Node holds a trace of fencing 1.0: the label or a pod of
// version 1. A node that fencing 1.0 stopped is not Ready, and its traces stay until
// it starts again.
func handleMigrationFrom10(_ context.Context, input *go_hook.HookInput) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[fencing10Node](input.Snapshots, fencing10NodesSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal '%s' snapshot: %w", fencing10NodesSnapshot, err)
	}

	podNodes, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, fencing10PodsSnapshot)
	if err != nil {
		return fmt.Errorf("failed to unmarshal '%s' snapshot: %w", fencing10PodsSnapshot, err)
	}

	found := slices.ContainsFunc(nodes, func(n fencing10Node) bool {
		return n.Ready && (n.Label || slices.Contains(podNodes, n.Name))
	})

	input.Values.Set("nodeManager.internal.fencing.migrationFrom10", found)
	return nil
}
