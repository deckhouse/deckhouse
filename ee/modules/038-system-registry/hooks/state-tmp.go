/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	registryStaticPodVersionAnnotation = "registry.deckhouse.io/config-version"
)

type registryStaticPodObject struct {
	registryStaticPod
	Node string
}

type registryStaticPod struct {
	IP      string
	Ready   bool
	Version string
}

type registryNode struct {
	IP     string
	Ready  bool
	Master bool
	Pods   map[string]registryStaticPod
}

type registryState struct {
	StaticPodVersion string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/state-tmp",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "static_pods",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":  "deckhouse",
					"app":       "system-registry",
					"module":    "system-registry",
					"component": "system-registry",
					"type":      "static-pod",
				},
			},
			FilterFunc: filterRegistryStaticPods,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: filterRegistryNodes,
		},
		{
			Name:       "state",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-state-tmp"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: filterRegistryState,
		},
	},
}, handleRegistryStaticPods)

func filterRegistryState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to convert state secret to struct: %v", err)
	}

	ret := registryState{
		StaticPodVersion: string(secret.Data["staticpod_version"]),
	}

	return ret, nil
}

func filterRegistryNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return "", fmt.Errorf("failed to convert node to struct: %v", err)
	}

	isReady := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			isReady = true
			break
		}
	}

	nodeObject := registryNode{
		Ready: isReady,
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			nodeObject.IP = addr.Address
			break
		}
	}

	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		nodeObject.Master = true
	}

	ret := helpers.NewKeyValue(node.Name, nodeObject)
	return ret, nil
}

func filterRegistryStaticPods(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1core.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return "", fmt.Errorf("failed to convert pod to struct: %v", err)
	}

	nodeFound := false
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "Node" {
			nodeFound = true
			break
		}
	}

	if !nodeFound {
		return "", nil
	}

	isReady := false
	for _, cond := range pod.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			isReady = true
			break
		}
	}

	podObject := registryStaticPodObject{
		registryStaticPod: registryStaticPod{
			IP:      pod.Status.PodIP,
			Ready:   isReady,
			Version: pod.Annotations[registryStaticPodVersionAnnotation],
		},
		Node: pod.Spec.NodeName,
	}

	ret := helpers.NewKeyValue(pod.Name, podObject)
	return ret, nil
}

func handleRegistryStaticPods(input *go_hook.HookInput) error {
	state, err := helpers.SnapshotToSingle[registryState](input, "state")
	if err != nil {
		return fmt.Errorf("cannot load state: %w", err)
	}

	nodes, err := helpers.SnapshotToMap[string, registryNode](input, "nodes")
	if err != nil {
		return fmt.Errorf("cannot load nodes: %w", err)
	}

	pods, err := helpers.SnapshotToMap[string, registryStaticPodObject](input, "static_pods")
	if err != nil {
		return fmt.Errorf("cannot load static pods: %w", err)
	}

	for name, pod := range pods {
		if node, ok := nodes[pod.Node]; ok {
			if node.Pods == nil {
				node.Pods = make(map[string]registryStaticPod)
			}
			node.Pods[name] = pod.registryStaticPod
		} else {
			input.Logger.Warn(
				"Node not found for static pod",
				"node", pod.Node,
				"pod", name,
			)
		}
	}

	if state.StaticPodVersion == "" {
		state.StaticPodVersion = "unknown"
	}

	input.Values.Set("systemRegistry.internal.state.nodes", nodes)
	input.Values.Set("systemRegistry.internal.state.staticpod_version", state.StaticPodVersion)

	return nil
}
