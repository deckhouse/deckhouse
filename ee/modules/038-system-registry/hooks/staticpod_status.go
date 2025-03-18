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
	"gopkg.in/yaml.v3"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	registryStaticPodVersionAnnotation = "registry.deckhouse.io/config-version"
)

type registryStaticPod struct {
	Name    string
	IP      string
	IsReady bool
	Version string
}

type registryStaticPodObject struct {
	registryStaticPod
	NodeName string
	NodeIP   string
}

type registryNodeObject struct {
	Name     string
	IP       string
	IsReady  bool
	IsMaster bool
}

type registryNode struct {
	registryNodeObject
	Pods []registryStaticPod
}

type registryState struct {
	StaticPodVersion string
	BashibleVersion  string
	Messages         []string
}

type registryConfig struct {
	Mode       string
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
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
			FilterFunc: filterRegistryNodes,
		},
		{
			Name:       "state",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-state"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: filterRegistryState,
		},
		{
			Name:       "config",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"registry-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: filterRegistryConfig,
		},
	},
}, handleRegistryStaticPods)

func filterRegistryConfig(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to convert config secret to struct: %v", err)
	}

	config := registryConfig{
		Mode:       string(secret.Data["mode"]),
		ImagesRepo: string(secret.Data["imagesRepo"]),
		UserName:   string(secret.Data["username"]),
		Password:   string(secret.Data["password"]),
		TTL:        string(secret.Data["ttl"]),
	}

	return config, nil
}

func filterRegistryState(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1core.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return "", fmt.Errorf("failed to convert state secret to struct: %v", err)
	}

	ret := registryState{
		StaticPodVersion: string(secret.Data["static_pod_version"]),
		BashibleVersion:  string(secret.Data["bashible_version"]),
	}

	if messagesData, ok := secret.Data["messages"]; ok {
		var messages []string

		err := yaml.Unmarshal(messagesData, &messages)
		if err != nil {
			return "", fmt.Errorf("cannot unmashal messages: %w", err)
		}

		ret.Messages = messages
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

	ret := registryNodeObject{
		Name:    node.Name,
		IsReady: isReady,
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			ret.IP = addr.Address
			break
		}
	}

	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		ret.IsMaster = true
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

	ret := registryStaticPodObject{
		registryStaticPod: registryStaticPod{
			Name:    pod.Name,
			IP:      pod.Status.PodIP,
			IsReady: isReady,
			Version: pod.Annotations[registryStaticPodVersionAnnotation],
		},
		NodeName: pod.Spec.NodeName,
		NodeIP:   pod.Status.HostIP,
	}

	return ret, nil
}

func handleRegistryStaticPods(input *go_hook.HookInput) error {
	podSnaps := input.Snapshots["static_pods"]
	nodeSnaps := input.Snapshots["nodes"]
	stateSnaps := input.Snapshots["state"]
	configSnaps := input.Snapshots["config"]

	var (
		state  registryState
		config registryConfig
	)

	if len(stateSnaps) == 1 {
		state = stateSnaps[0].(registryState)
	} else {
		msg := fmt.Sprintf("State snaps count: %v", len(stateSnaps))
		state.Messages = append(state.Messages, msg)
		input.Logger.Warn(msg)
	}

	if len(configSnaps) == 1 {
		config = configSnaps[0].(registryConfig)
	} else {
		msg := fmt.Sprintf("Config snaps count: %v", len(configSnaps))
		state.Messages = append(state.Messages, msg)
		input.Logger.Warn(msg)
	}

	nodes := make(map[string]registryNode)

	for _, snap := range nodeSnaps {
		nodeObject := snap.(registryNodeObject)
		node := registryNode{
			registryNodeObject: nodeObject,
		}
		nodes[node.Name] = node
	}

	for _, snap := range podSnaps {
		pod := snap.(registryStaticPodObject)

		if node, ok := nodes[pod.NodeName]; ok {
			node.Pods = append(node.Pods, pod.registryStaticPod)
			nodes[node.Name] = node
		} else {
			msg := fmt.Sprintf("Node \"%v\" not found for static pod \"%v\"", pod.NodeName, pod.Name)
			state.Messages = append(state.Messages, msg)
			input.Logger.Warn(
				msg,
				"node", pod.NodeName,
				"pod", pod.Name,
			)
		}
	}

	if state.StaticPodVersion == "" {
		state.StaticPodVersion = "unknown"
	}

	if state.BashibleVersion == "" {
		state.BashibleVersion = "unknown"
	}

	input.Values.Set("systemRegistry.internal.state.nodes", nodes)
	input.Values.Set("systemRegistry.internal.state.config", config)
	input.Values.Set("systemRegistry.internal.state.static_pod_version", state.StaticPodVersion)
	input.Values.Set("systemRegistry.internal.state.bashible_version", state.BashibleVersion)

	if len(state.Messages) > 0 {
		if len(state.Messages) > 30 {
			state.Messages = state.Messages[:30]
		}

		input.Values.Set("systemRegistry.internal.state.messages", state.Messages)
	}

	return nil
}
