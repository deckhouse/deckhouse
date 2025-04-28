/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	nodesSnapName = "master-nodes"
	podsSnapName  = "static-pods"
)

func snapName(prefix, name string) string {
	return fmt.Sprintf("%s-->%s", prefix, name)
}

func KubernetsConfig(name string) []go_hook.KubernetesConfig {
	ret := []go_hook.KubernetesConfig{
		{
			Name:          snapName(name, nodesSnapName),
			ApiVersion:    "v1",
			Kind:          "Node",
			LabelSelector: MasterNodeLabelSelector,
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var node v1core.Node

				err := sdk.FromUnstructured(obj, &node)
				if err != nil {
					return nil, fmt.Errorf("failed to convert node to struct: %v", err)
				}

				isReady := false
				for _, cond := range node.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						isReady = true
						break
					}
				}

				nodeObject := Node{
					Ready: isReady,
				}

				for _, addr := range node.Status.Addresses {
					if addr.Type == "InternalIP" {
						nodeObject.IP = addr.Address
						break
					}
				}

				ret := helpers.NewKeyValue(node.Name, nodeObject)
				return ret, nil
			},
		},
		{
			Name:              snapName(name, podsSnapName),
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: helpers.NamespaceSelector,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":  "deckhouse",
					"app":       "system-registry",
					"module":    "system-registry",
					"component": "system-registry",
					"type":      "node-services",
				},
			},
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var pod v1core.Pod

				err := sdk.FromUnstructured(obj, &pod)
				if err != nil {
					return nil, fmt.Errorf("failed to convert pod to struct: %v", err)
				}

				nodeFound := false
				for _, ref := range pod.OwnerReferences {
					if ref.Kind == "Node" {
						nodeFound = true
						break
					}
				}

				if !nodeFound {
					return nil, nil
				}

				isReady := false
				for _, cond := range pod.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						isReady = true
						break
					}
				}

				podObject := hookPod{
					Pod: Pod{
						Ready:   isReady,
						Version: pod.Annotations[PodVersionAnnotation],
					},
					Node: pod.Spec.NodeName,
				}

				ret := helpers.NewKeyValue(pod.Name, podObject)
				return ret, nil
			},
		},
		// TODO: add node configs hook
	}

	return ret
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	var (
		ret Inputs
		err error
	)

	ret.Nodes, err = helpers.SnapshotToMap[string, Node](input, snapName(name, nodesSnapName))
	if err != nil {
		return ret, fmt.Errorf("get Nodes snapshot error: %w", err)
	}

	pods, err := helpers.SnapshotToMap[string, hookPod](input, snapName(name, podsSnapName))
	if err != nil {
		return ret, fmt.Errorf("get Pods snapshot error: %w", err)
	}

	for name, pod := range pods {
		node, ok := ret.Nodes[pod.Node]
		if !ok {
			return ret, fmt.Errorf("cannot find Node \"%s\" for Pod \"%s\"", pod.Node, name)
		}

		if node.Pods == nil {
			node.Pods = make(NodePods)
		}
		node.Pods[name] = pod.Pod

		ret.Nodes[pod.Node] = node
	}

	return ret, nil
}
