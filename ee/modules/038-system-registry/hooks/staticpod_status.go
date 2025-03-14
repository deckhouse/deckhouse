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
)

type registryStaticPod struct {
	Name     string
	IP       string
	NodeName string
	NodeIP   string
	IsReady  bool
}

type registryMasterNode struct {
	Name    string
	IP      string
	IsReady bool
	Pods    []registryStaticPod
}

type registryState struct {
	Version string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/system-registry/staticpod-status",
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
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
			FilterFunc: filterRegistryMasterNodes,
		},
		{
			Name:       "state",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-deckhouse-state"},
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
		return "", fmt.Errorf("failed to convert secret to struct: %v", err)
	}

	ret := registryState{
		Version: string(secret.Data["version"]),
	}

	return ret, nil
}

func filterRegistryMasterNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

	ret := registryMasterNode{
		Name:    node.Name,
		IsReady: isReady,
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			ret.IP = addr.Address
			break
		}
	}

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

	ret := registryStaticPod{
		Name:     pod.Name,
		IP:       pod.Status.PodIP,
		NodeName: pod.Spec.NodeName,
		NodeIP:   pod.Status.HostIP,
		IsReady:  isReady,
	}

	return ret, nil
}

func handleRegistryStaticPods(input *go_hook.HookInput) error {
	podSnaps := input.Snapshots["static_pods"]
	nodeSnaps := input.Snapshots["nodes"]
	stateSnaps := input.Snapshots["state"]

	var state registryState

	if len(stateSnaps) > 0 {
		state = stateSnaps[0].(registryState)
	}

	nodes := make(map[string]registryMasterNode)

	for _, snap := range nodeSnaps {
		node := snap.(registryMasterNode)
		nodes[node.Name] = node
	}

	for _, snap := range podSnaps {
		pod := snap.(registryStaticPod)

		if node, ok := nodes[pod.NodeName]; ok {
			node.Pods = append(node.Pods, pod)
			nodes[node.Name] = node
		} else {
			input.Logger.Warn(
				"Node not found for static pod",
				"node", pod.NodeName,
				"pod", pod.Name,
			)
		}
	}

	input.Values.Set("systemRegistry.internal.state.masterNodes", nodes)
	input.Values.Set("systemRegistry.internal.state.version", state.Version)

	return nil
}
