/*
Copyright 2021 Flant CJSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1alpha1"
)

// get_node_user_crds retrieves all NodeUser CRs, checks uniqueness of UIDs and
// sets nodeManager.internal.nodeUsers value.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "nodeuser",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeUser",
			FilterFunc: getNodeUserCRDFilter,
		},
	},
}, getNodeUserCRDsHandler)

type NodeUserInfo struct {
	Name string                `json:"name"`
	Spec v1alpha1.NodeUserSpec `json:"spec"`
}

func getNodeUserCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	nodeUser := new(v1alpha1.NodeUser)
	err := sdk.FromUnstructured(obj, nodeUser)
	if err != nil {
		return nil, err
	}

	return NodeUserInfo{
		Name: nodeUser.Name,
		Spec: nodeUser.Spec,
	}, nil
}

func getNodeUserCRDsHandler(input *go_hook.HookInput) error {
	nodeUsers := input.Snapshots["nodeuser"]
	if len(nodeUsers) == 0 {
		nodeUsers = make([]go_hook.FilterResult, 0)
	}

	unique := true
	idx := make(map[int32][]string)
	for _, nodeUser := range nodeUsers {
		info := nodeUser.(NodeUserInfo)
		uid := info.Spec.UID
		if _, has := idx[uid]; !has {
			idx[uid] = make([]string, 0)
		}

		idx[uid] = append(idx[uid], info.Name)

		if len(idx[uid]) > 1 {
			unique = false
		}
	}

	// Prepare error message with NodeUser names.
	if !unique {
		msgs := make([]string, 0)
		for uid, names := range idx {
			if len(names) > 1 {
				msgs = append(msgs, fmt.Sprintf("%d in %v", uid, names))
			}
		}
		return fmt.Errorf("UIDs are not unique among NodeUser CRs: %s", strings.Join(msgs, ", "))
	}

	input.Values.Set("nodeManager.internal.nodeUsers", nodeUsers)
	return nil
}
