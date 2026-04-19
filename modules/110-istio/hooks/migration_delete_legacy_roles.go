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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

// This hook deletes legacy roles and rolebindings created by operator (not DH) in both scopes
// TODO: Remove this hook after 1.67

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "role_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "Role",
			FilterFunc: applyRoleFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
		{
			Name:       "rolebinding_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			FilterFunc: applyRoleBindingFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
		{
			Name:       "clusterrole_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyClusterRoleFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
		},
		{
			Name:       "clusterrolebinding_for_delete",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: applyClusterRoleBindingFilter,
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"release": "istio",
				},
			},
		},
	},
}, deleteLegacyRBACs)

func applyRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return objectInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

func applyRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return objectInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

func applyClusterRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func applyClusterRoleBindingFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func deleteLegacyRBACs(_ context.Context, input *go_hook.HookInput) error {
	// remove legacy Roles
	for role, err := range sdkobjectpatch.SnapshotIter[objectInfo](input.Snapshots.Get("role_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'role_for_delete' snapshot: %w", err)
		}

		input.Logger.Info("remove legacy Role %s in %s namespace", role.Name, role.Namespace)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "Role", role.Namespace, role.Name)
	}

	// remove legacy RoleBindings
	for roleBinding, err := range sdkobjectpatch.SnapshotIter[objectInfo](input.Snapshots.Get("rolebinding_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'rolebinding_for_delete' snapshot: %w", err)
		}

		input.Logger.Info("remove legacy RoleBinding %s in %s namespace", roleBinding.Name, roleBinding.Namespace)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", roleBinding.Namespace, roleBinding.Name)
	}

	// remove legacy ClusterRoles
	for clusterRoleName, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get("clusterrole_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'clusterrole_for_delete' snapshot: %w", err)
		}

		input.Logger.Info("remove legacy ClusterRole %s", clusterRoleName)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRole", "", clusterRoleName)
	}

	// remove legacy ClusterRoleBindings
	for clusterRoleBindingName, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get("clusterrolebinding_for_delete")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'clusterrolebinding_for_delete' snapshot: %w", err)
		}

		input.Logger.Info("remove legacy ClusterRoleBinding %s", clusterRoleBindingName)
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", clusterRoleBindingName)
	}

	return nil
}

type objectInfo struct {
	Name      string
	Namespace string
}
