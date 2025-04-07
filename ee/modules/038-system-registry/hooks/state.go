/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v3"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers/submodule"
)

const (
	registryStaticPodVersionAnnotation = "registry.deckhouse.io/config-version"
)

type registryStaticPod struct {
	Name    string
	IP      string
	Ready   bool
	Version string
}

type registryStaticPodObject struct {
	registryStaticPod
	Node string
}

type registryNodeObject struct {
	Name   string
	IP     string
	Ready  bool
	Master bool
}

type registryNode struct {
	registryNodeObject
	Pods []registryStaticPod
}

type registryState struct {
	StaticPodVersion string
	BashibleVersion  string
	Messages         []string
	PkiMode          string
	Users            []string
	UsersEnabled     bool
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
	Queue:        "/modules/system-registry/state",
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
		StaticPodVersion: string(secret.Data["staticpod_version"]),
		BashibleVersion:  string(secret.Data["bashible_version"]),
		PkiMode:          string(secret.Data["pki_mode"]),
	}

	if messagesData, ok := secret.Data["messages"]; ok {
		var messages []string

		err := yaml.Unmarshal(messagesData, &messages)
		if err != nil {
			return "", fmt.Errorf("cannot unmashal messages: %w", err)
		}

		ret.Messages = messages
	}

	userEnabled := string(secret.Data["users_enabled"])
	userEnabled = strings.TrimSpace(userEnabled)
	userEnabled = strings.ToLower(userEnabled)
	ret.UsersEnabled = userEnabled == "true"

	users := strings.Split(string(secret.Data["users"]), ",")
	usersMap := make(map[string]struct{})

	for _, user := range users {
		user = strings.TrimSpace(user)
		user = strings.ToLower(user)

		if user == "" {
			continue
		}

		usersMap[user] = struct{}{}
	}

	for user := range usersMap {
		ret.Users = append(ret.Users, user)
	}

	sort.Strings(ret.Users)

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
		Name:  node.Name,
		Ready: isReady,
	}

	for _, addr := range node.Status.Addresses {
		if addr.Type == "InternalIP" {
			ret.IP = addr.Address
			break
		}
	}

	if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
		ret.Master = true
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
			Ready:   isReady,
			Version: pod.Annotations[registryStaticPodVersionAnnotation],
		},
		Node: pod.Spec.NodeName,
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

		if node, ok := nodes[pod.Node]; ok {
			node.Pods = append(node.Pods, pod.registryStaticPod)
			nodes[node.Name] = node
		} else {
			msg := fmt.Sprintf("Node \"%v\" not found for static pod \"%v\"", pod.Node, pod.Name)
			state.Messages = append(state.Messages, msg)
			input.Logger.Warn(
				msg,
				"node", pod.Node,
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
	input.Values.Set("systemRegistry.internal.state.staticpod_version", state.StaticPodVersion)
	input.Values.Set("systemRegistry.internal.state.bashible_version", state.BashibleVersion)

	if state.PkiMode != "" {
		input.Values.Set("systemRegistry.internal.pki.mode", state.PkiMode)
	}

	if len(state.Messages) > 0 {
		if len(state.Messages) > 30 {
			state.Messages = state.Messages[:30]
		}

		input.Values.Set("systemRegistry.internal.state.messages", state.Messages)
	}

	input.Values.Set("systemRegistry.internal.state.users.enabled", state.UsersEnabled)

	if len(state.Users) > 0 {
		input.Values.Set("systemRegistry.internal.state.users.users", state.Users)
	} else {
		input.Values.Remove("systemRegistry.internal.state.users.users")
	}

	if !state.UsersEnabled {
		submodule.DisableSubmodule(input.Values, "users")
	} else {
		usersVer, err := submodule.SetSubmoduleConfig(input.Values, "users", state.Users)
		if err != nil {
			return fmt.Errorf("cannot set users config: %w", err)
		}

		input.Values.Set("systemRegistry.internal.state.users.version", usersVer)
	}

	return nil
}
