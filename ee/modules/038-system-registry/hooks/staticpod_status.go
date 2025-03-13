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
	PodName  string
	PodIP    string
	NodeName string
	NodeIP   string
	IsReady  bool
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
					"app":       "system-registry",
					"module":    "system-registry",
					"component": "system-registry",
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
			FilterFunc: nil,
		},
	},
}, handleRegistryStaticPods)

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
		PodName:  pod.Name,
		PodIP:    pod.Status.PodIP,
		NodeName: pod.Spec.NodeName,
		NodeIP:   pod.Status.HostIP,
		IsReady:  isReady,
	}

	return ret, nil
}

func handleRegistryStaticPods(input *go_hook.HookInput) error {
	pods := input.Snapshots["static_pods"]
	nodes := input.Snapshots["nodes"]

	input.Values.Set("systemRegistry.internal.state.staticPods", pods)
	input.Values.Set("systemRegistry.internal.state.masterNodes", nodes)

	return nil
}
