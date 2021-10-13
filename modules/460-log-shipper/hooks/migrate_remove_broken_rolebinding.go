/*
Copyright 2021 Flant JSC

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

// Migration 05.08.2021: Remove after this commit (265ebbf0d116f141b5319bc62b62b72c9b32c43e) reached RockSolid

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

func applyObjFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	roleBinding := &rbacv1.RoleBinding{}
	err := sdk.FromUnstructured(obj, roleBinding)
	if err != nil {
		return nil, fmt.Errorf("cannot create rolebinding object: %v", err)
	}
	return roleBinding.RoleRef.Name == "d8:log-shipper", nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},

	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "log_shipper_rolebinding",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-log-shipper"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"log-shipper"},
			},
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   applyObjFilter,
		},
	},
}, removeProblemRoleBinding)

func removeProblemRoleBinding(input *go_hook.HookInput) error {

	roleBindingTypeSnap := input.Snapshots["log_shipper_rolebinding"]
	if len(roleBindingTypeSnap) > 0 {
		if roleBindingTypeSnap[0].(bool) {
			input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", "d8-log-shipper", "log-shipper")
		}
	}

	return nil
}
