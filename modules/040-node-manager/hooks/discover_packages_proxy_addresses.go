/*
Copyright 2024 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	packagesProxyPort = 4219
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "packages_proxy",
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "registry-packages-proxy",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: packagesProxyPodFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: packagesProxyNodeFilter,
		},
	},
}, handlePackagesProxyEndpoints)

type packagesProxyPodInfo struct {
	HostIP   string `json:"hostIP"`
	NodeName string `json:"nodeName"`
}

type packagesProxyNodeInfo struct {
	Name            string `json:"name"`
	IsReady         bool   `json:"isReady"`
	IsDeleting      bool   `json:"isDeleting"`
	IsUnschedulable bool   `json:"isUnschedulable"`
	IsManaged       bool   `json:"isManaged"`
	IsControlPlane  bool   `json:"isControlPlane"`
}

func packagesProxyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var isReady bool

	pod := &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}
	if !isReady {
		return nil, nil
	}
	return packagesProxyPodInfo{
		HostIP:   pod.Status.HostIP,
		NodeName: pod.Spec.NodeName,
	}, nil
}

func packagesProxyNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var isReady bool

	node := &corev1.Node{}
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("cannot parse node object from unstructured: %v", err)
	}

	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	_, hasControlPlaneLabel := node.Labels["node-role.kubernetes.io/control-plane"]
	_, hasMasterLabel := node.Labels["node-role.kubernetes.io/master"]
	_, hasNodeGroupLabel := node.Labels["node.deckhouse.io/group"]

	return packagesProxyNodeInfo{
		Name:            node.Name,
		IsReady:         isReady,
		IsDeleting:      node.DeletionTimestamp != nil,
		IsUnschedulable: node.Spec.Unschedulable,
		IsManaged:       hasNodeGroupLabel,
		IsControlPlane:  hasControlPlaneLabel || hasMasterLabel,
	}, nil
}

func handlePackagesProxyEndpoints(_ context.Context, input *go_hook.HookInput) error {
	nodeByName, err := collectPackagesProxyNodes(input)
	if err != nil {
		return err
	}

	endpointsSet, fallbackEndpointsSet, err := collectPackagesProxyEndpoints(input, nodeByName)
	if err != nil {
		return err
	}

	if endpointsSet.Size() == 0 && fallbackEndpointsSet.Size() > 0 {
		endpointsSet = fallbackEndpointsSet
	}

	endpointsList := endpointsSet.Slice() // sorted

	if len(endpointsList) == 0 {
		return fmt.Errorf("no packages proxy endpoints found")
	}

	input.Logger.Info("found packages proxy endpoints", slog.String("endpoints", strings.Join(endpointsList, ",")))
	input.Values.Set("nodeManager.internal.packagesProxy.addresses", endpointsList)

	return nil
}

func collectPackagesProxyNodes(input *go_hook.HookInput) (map[string]packagesProxyNodeInfo, error) {
	nodeByName := make(map[string]packagesProxyNodeInfo)

	for node, err := range sdkobjectpatch.SnapshotIter[packagesProxyNodeInfo](input.Snapshots.Get("nodes")) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'nodes' snapshots: %v", err)
		}
		nodeByName[node.Name] = node
	}

	return nodeByName, nil
}

func collectPackagesProxyEndpoints(input *go_hook.HookInput, nodeByName map[string]packagesProxyNodeInfo) (set.Set, set.Set, error) {
	endpointsSet := set.New()
	fallbackEndpointsSet := set.New()

	for pod, err := range sdkobjectpatch.SnapshotIter[packagesProxyPodInfo](input.Snapshots.Get("packages_proxy")) {
		if err != nil {
			return nil, nil, fmt.Errorf("failed to iterate over 'packages_proxy' snapshots: %v", err)
		}
		processPackagesProxyPod(pod, nodeByName, endpointsSet, fallbackEndpointsSet)
	}

	return endpointsSet, fallbackEndpointsSet, nil
}

func processPackagesProxyPod(
	pod packagesProxyPodInfo,
	nodeByName map[string]packagesProxyNodeInfo,
	endpointsSet set.Set,
	fallbackEndpointsSet set.Set,
) {
	if pod.NodeName == "" {
		return
	}

	node, ok := nodeByName[pod.NodeName]
	if !ok {
		return
	}

	if !node.IsReady || !node.IsManaged || node.IsUnschedulable {
		return
	}

	endpoint := fmt.Sprintf("%s:%d", pod.HostIP, packagesProxyPort)

	if !node.IsControlPlane {
		fallbackEndpointsSet.Add(endpoint)
		return
	}

	if node.IsDeleting {
		return
	}

	fallbackEndpointsSet.Add(endpoint)
	endpointsSet.Add(endpoint)
}
