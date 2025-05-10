// Copyright 2025 Flant JSC
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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const labelKey = "ingress-nginx-controller.deckhouse.io/with-failover-node"

type Node struct {
	Name       string
	Labels     map[string]string
	Conditions []corev1.NodeCondition
}

type Pod struct {
	Name     string
	NodeName string
	Labels   map[string]string
	Ready    bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ingress-nginx",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "proxyFailoverPods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "proxy-failover",
				},
			},
			FilterFunc: applyProxyFailoverPodFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNodeFilter,
		},
	},
}, setProxyFailoverLabel)

func applyProxyFailoverPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, err
	}

	isReady := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	return Pod{
		Name:     pod.Name,
		NodeName: pod.Spec.NodeName,
		Labels:   pod.Labels,
		Ready:    isReady,
	}, nil
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}
	return Node{
		Name:       node.Name,
		Labels:     node.Labels,
		Conditions: node.Status.Conditions,
	}, nil
}

func setProxyFailoverLabel(input *go_hook.HookInput) error {
	// Collect nodes that have a Ready proxy-failover Pod running
	nodesWithFailover := make(map[string]struct{}, len(input.Snapshots["proxyFailoverPods"])) // All active nodes

	for _, snap := range input.Snapshots["proxyFailoverPods"] {
		pod := snap.(Pod)
		if pod.Ready && pod.NodeName != "" {
			nodesWithFailover[pod.NodeName] = struct{}{}
		}
	}

	// Add the label to nodes that have a proxy-failover Pod and don't have the label yet
	for _, snap := range input.Snapshots["nodes"] {
		node := snap.(Node)

		_, podExists := nodesWithFailover[node.Name]
		_, labelExists := node.Labels[labelKey]

		if podExists {
			fmt.Printf("Adding label %q to node %q", labelKey, node.Name)
			input.PatchCollector.PatchWithMerge(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						labelKey: "true",
					},
				},
			}, "v1", "Node", "", node.Name)
		}

		// Change label value to false if node have not a proxy-failover Pod
		if labelExists && !podExists {
			fmt.Printf("Changed label %q to node %q on false value", labelKey, node.Name)
			input.PatchCollector.PatchWithMerge(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						labelKey: "false",
					},
				},
			}, "v1", "Node", "", node.Name)
		}
	}

	return nil
}
