/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager/node_user",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "node_user",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "NodeUser",
			FilterFunc:                   applyNodeUsersFilter,
		},
	},
}, addStatusSubresourceForNodeUser)

type existingStatus struct {
	UserName     string `json:"user_name"`
	StatusExists bool   `json:"status_exists"`
}

func applyNodeUsersFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	_, exists, err := unstructured.NestedFieldNoCopy(obj.Object, "status")
	if err != nil {
		return nil, err
	}

	return existingStatus{
		UserName:     obj.GetName(),
		StatusExists: exists,
	}, nil
}

func addStatusSubresourceForNodeUser(input *go_hook.HookInput) error {
	nodeUserSnap := input.Snapshots["node_user"]
	if len(nodeUserSnap) == 0 {
		return nil
	}

	for _, item := range nodeUserSnap {
		nu := item.(existingStatus)
		if nu.StatusExists {
			input.LogEntry.Debugf("Status already exists for node user %s", nu.UserName)
			continue
		}

		input.LogEntry.Infof("Add status for node user %s", nu.UserName)

		input.PatchCollector.Filter(func(u *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			objCopy := u.DeepCopy()
			status := map[string]interface{}{
				"errors": make(map[string]interface{}),
			}
			err := unstructured.SetNestedField(objCopy.Object, status, "status")
			if err != nil {
				return nil, err
			}
			return objCopy, nil
		}, "deckhouse.io/v1", "NodeUser", "", nu.UserName)
	}

	return nil
}
