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
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/user-authz/handle-scope-bindings",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "manageBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: filterClusterRoleBinding,
		},
		{
			Name:       "useBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":                    "deckhouse",
					"rbac.deckhouse.io/automated": "true",
				},
			},
			FilterFunc: filterRoleBinding,
		},
		{
			Name:       "scopeManageRoles",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":                "deckhouse",
					"rbac.deckhouse.io/kind":  "manage",
					"rbac.deckhouse.io/level": "scope",
				},
			},
			FilterFunc: filterScopeManageRole,
		},
		{
			Name:       "moduleManageRoles",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":                "deckhouse",
					"rbac.deckhouse.io/kind":  "manage",
					"rbac.deckhouse.io/level": "module",
				},
			},
			FilterFunc: filterModuleManageRole,
		},
	},
}, syncRoles)

type filteredBinding struct {
	Name        string           `json:"name"`
	Namespace   string           `json:"namespace"`
	RelatedWith string           `json:"relatedWith"`
	RoleName    string           `json:"roleRef"`
	Subjects    []rbacv1.Subject `json:"subjects"`
}

func filterClusterRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &clusterRoleBinding); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(clusterRoleBinding.RoleRef.Name, "d8:manage:") {
		return nil, nil
	}
	return &filteredBinding{
		Name:     clusterRoleBinding.Name,
		RoleName: clusterRoleBinding.RoleRef.Name,
		Subjects: clusterRoleBinding.Subjects,
	}, nil
}
func filterRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var roleBinding rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &roleBinding); err != nil {
		return nil, err
	}
	var relatedWith string
	for key, val := range roleBinding.Labels {
		if key == "rbac.deckhouse.io/related-with" {
			relatedWith = val
		}
	}
	return &filteredBinding{
		Name:        roleBinding.Name,
		Namespace:   roleBinding.Namespace,
		RelatedWith: relatedWith,
		RoleName:    roleBinding.RoleRef.Name,
		Subjects:    roleBinding.Subjects,
	}, nil
}

type filteredScopeRole struct {
	Name  string `json:"name"`
	Scope string `json:"scope"`
	Role  string `json:"role"`
}

func filterScopeManageRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var clusterRole rbacv1.ClusterRole
	if err := sdk.FromUnstructured(obj, &clusterRole); err != nil {
		return nil, err
	}
	return &filteredScopeRole{
		Name:  clusterRole.Name,
		Scope: clusterRole.Labels["rbac.deckhouse.io/scope"],
		Role:  clusterRole.Labels["rbac.deckhouse.io/aggregate-to-all-as"],
	}, nil
}

type filteredModuleRole struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	Namespace string   `json:"namespace"`
}

func filterModuleManageRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var clusterRole rbacv1.ClusterRole
	if err := sdk.FromUnstructured(obj, &clusterRole); err != nil {
		return nil, err
	}
	var role filteredModuleRole
	role.Name = clusterRole.Name
	for key, val := range clusterRole.Labels {
		if roleScope, found := strings.CutPrefix(key, "rbac.deckhouse.io/aggregate-to-"); found && val == "true" {
			role.Scopes = append(role.Scopes, roleScope)
		}
		if key == "rbac.deckhouse.io/namespace" {
			role.Namespace = val
		}
	}
	return &role, nil
}

func syncRoles(input *go_hook.HookInput) error {
	var bindings []*filteredBinding
	for _, binding := range input.Snapshots["manageBindings"] {
		if binding != nil {
			bindings = append(bindings, expectedRoleBindings(input.Snapshots, binding.(*filteredBinding))...)
		}
	}
	ensureBindings(input, bindings)
	return nil
}

func expectedRoleBindings(snapshots go_hook.Snapshots, manageBinding *filteredBinding) []*filteredBinding {
	var expectedUseBindings []*filteredBinding
	for _, sr := range snapshots["scopeManageRoles"] {
		if sr != nil {
			scopeRole := sr.(*filteredScopeRole)
			if scopeRole.Name == manageBinding.RoleName {
				for _, mr := range snapshots["moduleManageRoles"] {
					if mr != nil {
						moduleRole := mr.(*filteredModuleRole)
						if slices.Contains(moduleRole.Scopes, scopeRole.Scope) {
							expectedUseBindings = append(expectedUseBindings, &filteredBinding{
								Name:        fmt.Sprintf("d8:binding:%s", manageBinding.Name),
								Namespace:   moduleRole.Namespace,
								RelatedWith: manageBinding.Name,
								RoleName:    fmt.Sprintf("d8:use:role:%s", scopeRole.Role),
								Subjects:    manageBinding.Subjects,
							})
						}
					}
				}
			}
		}
	}
	return expectedUseBindings
}

func ensureBindings(input *go_hook.HookInput, expectedUseBindings []*filteredBinding) {
	var foundBindings = make(map[string]string)
	for _, expected := range expectedUseBindings {
		var found bool
		for _, existing := range input.Snapshots["useBindings"] {
			if existing != nil && reflect.DeepEqual(expected, existing.(*filteredBinding)) {
				found = true
				foundBindings[fmt.Sprintf("%s-%s", expected.Name, expected.Name)] = expected.Name
				break
			}
		}
		if !found {
			input.PatchCollector.Create(buildBinding(expected), object_patch.UpdateIfExists())
		}
	}
	for _, existing := range input.Snapshots["useBindings"] {
		if existing != nil {
			binding := existing.(*filteredBinding)
			if _, found := foundBindings[fmt.Sprintf("%s-%s", binding.Name, binding.Name)]; !found {
				input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", binding.Namespace, binding.Name)
			}
		}
	}
}

func buildBinding(filtered *filteredBinding) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      filtered.Name,
			Namespace: filtered.Namespace,
			Labels: map[string]string{
				"heritage":                       "deckhouse",
				"rbac.deckhouse.io/automated":    "true",
				"rbac.deckhouse.io/related-with": filtered.RelatedWith,
			},
		},
		Subjects: filtered.Subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io/v1",
			Kind:     "ClusterRole",
			Name:     filtered.RoleName,
		},
	}
}
