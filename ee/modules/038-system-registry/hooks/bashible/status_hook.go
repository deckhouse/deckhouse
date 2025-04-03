/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package bashible

import (
	"fmt"

	bashible_input "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models/input"
	bashible_status "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/bashible/models/status"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	BashibleVersionNodeAnnotation = "registry.deckhouse.io/version"
)

type NodeInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func BashibleStatusHook(order float64, queue string) bool {
	const (
		snapNodesInfo = "nodesInfo"
	)

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: order},
		Queue:        queue,
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       snapNodesInfo,
				ApiVersion: "v1",
				Kind:       "Node",
				FilterFunc: FilterNodeInfo,
			},
		},
	}, func(hookInput *go_hook.HookInput) error {
		inputData, err := bashible_input.Get(hookInput)
		if err != nil {
			return fmt.Errorf("failed to get input data: %w", err)
		}

		// If no input data is available, remove the existing bashible status
		if inputData == nil {
			bashible_status.Remove(hookInput)
			return nil
		}

		inputVersion := inputData.Version
		bashibleStatus := bashible_status.Status{
			Ready:   true,
			Version: inputVersion,
			Nodes:   map[string]bashible_status.NodeStatus{},
		}

		for _, nodeInfoSnap := range hookInput.Snapshots[snapNodesInfo] {
			nodeInfo := nodeInfoSnap.(NodeInfo)
			nodeStatus := bashible_status.NodeStatus{
				Version: nodeInfo.Version,
				Ready:   nodeInfo.Version == inputVersion,
			}

			if !nodeStatus.Ready {
				bashibleStatus.Ready = false
			}

			bashibleStatus.Nodes[nodeInfo.Name] = nodeStatus
		}

		if !bashibleStatus.Ready {
			bashibleStatus.Version = registry_const.UnknownVersion
		}

		bashible_status.Set(hookInput, bashibleStatus)
		return nil
	})
}

func FilterNodeInfo(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node v1core.Node
	err := sdk.FromUnstructured(obj, &node)
	if err != nil {
		return nil, fmt.Errorf("failed to convert node \"%s\" to struct: %v", obj.GetName(), err)
	}

	ret := NodeInfo{
		Name: node.Name,
	}

	if version, ok := node.Annotations[BashibleVersionNodeAnnotation]; ok {
		ret.Version = version
	} else {
		ret.Version = registry_const.UnknownVersion
	}

	return ret, nil
}
