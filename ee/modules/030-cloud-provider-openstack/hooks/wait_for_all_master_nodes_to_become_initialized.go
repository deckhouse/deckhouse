/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 3 * time.Second,
		ExecutionBurst:       3,
	},
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{"node-role.kubernetes.io/control-plane": ""},
			},
			FieldSelector:                nil,
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterNodes,
		},
	},
}, handleAllMasterNodes)

func handleAllMasterNodes(_ context.Context, input *go_hook.HookInput) error {
	nodes, err := sdkobjectpatch.UnmarshalToStruct[uninitializedNode](input.Snapshots, "nodes")
	if err != nil {
		return fmt.Errorf("failed to unmarshal nodes snapshot: %w", err)
	}

	totalCount := len(nodes)
	var initializedCount int

	for _, node := range nodes {
		if node.Uninitialized {
			continue
		}
		initializedCount++
	}

	if initializedCount != totalCount {
		return errors.New("waiting for master nodes to become initialized by cloud provider")
	}

	return nil
}

func filterNodes(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var uninitialized bool

	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.cloudprovider.kubernetes.io/uninitialized" {
			uninitialized = true
			break
		}
	}

	return uninitializedNode{Name: node.Name, Uninitialized: uninitialized}, nil
}

type uninitializedNode struct {
	Name          string
	Uninitialized bool
}
