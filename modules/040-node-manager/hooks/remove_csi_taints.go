package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},

	Queue: "/modules/node-manager/remove_csi_taints",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "csinodes",
			WaitForSynchronization: pointer.BoolPtr(false),
			ApiVersion:             "storage.k8s.io/v1beta1",
			Kind:                   "CSINode",
			FilterFunc:             csiFilterCSINode, //  jqFilter: '{"name": .metadata.name}'
		},
		{
			Name:                         "nodes",
			WaitForSynchronization:       pointer.BoolPtr(false),
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   csiFilterNode, // '{"needPatch": ([(.spec.taints // [])[] | select(.key == "node.deckhouse.io/csi-not-bootstrapped")] | length > 0), "name": .metadata.name}',
		},
	},
}, handleRemoveCSI)

func csiFilterCSINode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}
func csiFilterNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	var needPatch bool
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node.deckhouse.io/csi-not-bootstrapped" {
			needPatch = true
			break
		}
	}

	return removeCSINode{
		Name:      node.Name,
		NeedPatch: needPatch,
	}, nil
}

type removeCSINode struct {
	Name      string
	NeedPatch bool
}

func handleRemoveCSI(input *go_hook.HookInput) error {
	nodes := make(map[string]bool)
	snap := input.Snapshots["nodes"]
	for _, sn := range snap {
		node := sn.(removeCSINode)
		nodes[node.Name] = node.NeedPatch
	}

	snap = input.Snapshots["csinodes"]
	for _, sn := range snap {
		csiName := sn.(string)

		needPatch, ok := nodes[csiName]
		if !ok {
			continue
		}
		if !needPatch {
			continue
		}

		err := input.ObjectPatcher.FilterObject(removeCSIFilterNode, "v1", "Node", "", csiName, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func removeCSIFilterNode(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var node *v1.Node

	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, err
	}

	taints := make([]v1.Taint, 0)

	for _, taint := range node.Spec.Taints {
		if taint.Key != "node.deckhouse.io/csi-not-bootstrapped" {
			taints = append(taints, taint)
		}
	}

	node.Spec.Taints = taints

	return sdk.ToUnstructured(node)
}
