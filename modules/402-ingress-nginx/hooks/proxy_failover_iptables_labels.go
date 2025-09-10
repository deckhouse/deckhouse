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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const labelKey = "ingress-nginx-controller.deckhouse.io/need-hostwithfailover-cleanup"

type Node struct {
	Name      string
	IsLabeled bool
}

type Pod struct {
	Name     string
	NodeName string
	Labels   map[string]string
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

	return Pod{
		Name:     pod.Name,
		NodeName: pod.Spec.NodeName,
		Labels:   pod.Labels,
	}, nil
}

func applyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	_, labeled := node.Labels[labelKey]

	return Node{
		Name:      node.Name,
		IsLabeled: labeled,
	}, nil
}

func setProxyFailoverLabel(_ context.Context, input *go_hook.HookInput) error {
	// Collect nodes that have a Ready proxy-failover Pod running
	nodesWithRunningFailover := make(map[string]struct{}, len(input.Snapshots.Get("proxyFailoverPods"))) // All active nodes

	for pod, err := range sdkobjectpatch.SnapshotIter[Pod](input.Snapshots.Get("proxyFailoverPods")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'proxyFailoverPods' snapshots: %w", err)
		}

		if pod.NodeName != "" {
			nodesWithRunningFailover[pod.NodeName] = struct{}{}
		}
	}

	// Add the label to nodes that have a proxy-failover Pod and don't have the label yet
	for node, err := range sdkobjectpatch.SnapshotIter[Node](input.Snapshots.Get("nodes")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
		}

		_, podExists := nodesWithRunningFailover[node.Name]

		if podExists {
			log.Info(fmt.Sprintf("Adding label %q to node %q", labelKey, node.Name))
			input.PatchCollector.PatchWithMerge(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						labelKey: "false",
					},
				},
			}, "v1", "Node", "", node.Name)
		}

		// Change label value to false if node have not a proxy-failover Pod
		if node.IsLabeled && !podExists {
			log.Info(fmt.Sprintf("Changed label %q to node %q on false value", labelKey, node.Name))
			input.PatchCollector.PatchWithMerge(map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{
						labelKey: "true",
					},
				},
			}, "v1", "Node", "", node.Name)
		}
	}

	return nil
}
