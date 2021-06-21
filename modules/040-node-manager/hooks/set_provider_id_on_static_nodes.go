package hooks

import (
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "nodes",
			WaitForSynchronization: pointer.BoolPtr(false),
			ApiVersion:             "v1",
			Kind:                   "Node",
			FilterFunc:             setProviderIDNodeFilter,
		},
	},
}, handleSetProviderID)

func setProviderIDNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var needPatch bool

	var hasUninitializedTaint bool
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.cloudprovider.kubernetes.io/uninitialized" {
			hasUninitializedTaint = true
			break
		}
	}

	if !hasUninitializedTaint && node.Spec.ProviderID == "" && node.Labels["node.deckhouse.io/type"] != "Cloud" {
		needPatch = true
	}

	return providerIDNode{
		Name:      node.Name,
		NeedPatch: needPatch,
	}, nil
}

type providerIDNode struct {
	Name      string
	NeedPatch bool
}

func handleSetProviderID(input *go_hook.HookInput) error {
	snap := input.Snapshots["nodes"]

	for _, sn := range snap {
		node := sn.(providerIDNode)
		if !node.NeedPatch {
			continue
		}

		err := input.ObjectPatcher.MergePatchObject(staticPatch, "v1", "Node", "", node.Name, "")
		if err != nil {
			return err
		}
	}

	return nil
}

var (
	staticPatch, _ = json.Marshal(
		map[string]interface{}{
			"spec": map[string]string{
				"providerID": "static://",
			},
		},
	)
)
