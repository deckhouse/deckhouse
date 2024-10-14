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
			FilterFunc: filterManageBinding,
		},
		{
			Name:       "manageRoles",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":               "deckhouse",
					"rbac.deckhouse.io/kind": "manage",
				},
			},
			FilterFunc: filterManageRole,
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
			FilterFunc: filterUseBinding,
		},
	},
}, syncRoles)

type filteredUseBinding struct {
	Name        string           `json:"name"`
	Namespace   string           `json:"namespace"`
	RelatedWith string           `json:"related_with"`
	RoleName    string           `json:"role_name"`
	Subjects    []rbacv1.Subject `json:"subjects"`
}

func filterUseBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var binding rbacv1.RoleBinding
	if err := sdk.FromUnstructured(obj, &binding); err != nil {
		return nil, err
	}
	if binding.Labels == nil || len(binding.Labels) == 0 {
		return nil, nil
	}
	relatedWith, ok := binding.Labels["rbac.deckhouse.io/related-with"]
	if !ok {
		return nil, nil
	}
	return &filteredUseBinding{
		Name:        binding.Name,
		Namespace:   binding.Namespace,
		RelatedWith: relatedWith,
		RoleName:    binding.RoleRef.Name,
		Subjects:    binding.Subjects,
	}, nil
}

type filteredManageBinding struct {
	Name     string           `json:"name"`
	RoleName string           `json:"role_name"`
	Subjects []rbacv1.Subject `json:"subjects"`
}

func filterManageBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var binding rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &binding); err != nil {
		return nil, err
	}
	return &filteredManageBinding{
		Name:     binding.Name,
		RoleName: binding.RoleRef.Name,
		Subjects: binding.Subjects,
	}, nil
}

type filteredManageRole struct {
	Name  string `json:"name"`
	Level string `json:"level"`
	// manage fields
	Scope     string `json:"scope"`
	Namespace string `json:"namespace"`
	// module cap fields
	Namespaces []string `json:"namespaces"`
	Scopes     []string `json:"scopes"`
}

func filterManageRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var role rbacv1.ClusterRole
	if err := sdk.FromUnstructured(obj, &role); err != nil {
		return nil, err
	}
	filtered := &filteredManageRole{
		Name: role.Name,
	}
	for key, val := range role.Labels {
		switch key {
		case "rbac.deckhouse.io/namespace":
			filtered.Namespace = val
		case "rbac.deckhouse.io/scope":
			filtered.Scope = val
		case "rbac.deckhouse.io/level":
			filtered.Level = val
		}
		if involved, ok := strings.CutPrefix(key, "rbac.deckhouse.io/aggregate-to-"); ok {
			involved, _ = strings.CutSuffix(involved, "-as")
			filtered.Scopes = append(filtered.Scopes, involved)
		}
	}
	if filtered.Level == "all" {
		filtered.Scope = "all"
	}
	return filtered, nil
}

func syncRoles(input *go_hook.HookInput) error {
	roles := parseRoles(input.Snapshots["manageRoles"])
	expected := make(map[string]*filteredUseBinding)
	for _, snapBinding := range input.Snapshots["manageBindings"] {
		binding := snapBinding.(*filteredManageBinding)
		namespaces, ok := roles[binding.RoleName]
		if !ok {
			continue
		}
		splits := strings.Split(binding.RoleName, ":")
		for _, namespace := range namespaces {
			expectedBinding := &filteredUseBinding{
				Name:        fmt.Sprintf("d8:use:binding:%s", binding.Name),
				Namespace:   namespace,
				RelatedWith: binding.Name,
				RoleName:    fmt.Sprintf("d8:use:role:%s", splits[len(splits)-1]),
				Subjects:    binding.Subjects,
			}
			input.PatchCollector.Create(createBinding(expectedBinding), object_patch.UpdateIfExists())
			expected[expectedBinding.Name] = expectedBinding
		}
	}

	// delete excess
	for _, existingSnap := range input.Snapshots["useBindings"] {
		if existingSnap != nil {
			continue
		}
		existing := existingSnap.(*filteredUseBinding)
		if _, ok := expected[existing.RoleName]; !ok {
			input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", existing.Namespace, existing.Name)
		}
	}
	return nil
}

func parseRoles(manageRoles []go_hook.FilterResult) map[string][]string {
	var scopes = make(map[string][]string)
	for _, snapRole := range manageRoles {
		if snapRole != nil {
			role := snapRole.(*filteredManageRole)
			if role.Level != "module" || role.Namespace == "" {
				continue
			}
			for _, scope := range role.Scopes {
				if !slices.Contains(scopes[scope], role.Namespace) {
					scopes[scope] = append(scopes[scope], role.Namespace)
				}
			}
			if !slices.Contains(scopes["all"], role.Namespace) {
				scopes["all"] = append(scopes["all"], role.Namespace)
			}
		}
	}

	var roles = make(map[string][]string)
	for _, snapRole := range manageRoles {
		if snapRole != nil {
			role := snapRole.(*filteredManageRole)
			if role.Level == "module" || role.Scope == "" {
				continue
			}
			namespaces, ok := scopes[role.Scope]
			if !ok {
				continue
			}
			roles[role.Name] = namespaces
		}
	}
	return roles
}

func createBinding(binding *filteredUseBinding) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      binding.Name,
			Namespace: binding.Namespace,
			Labels: map[string]string{
				"heritage":                       "deckhouse",
				"rbac.deckhouse.io/automated":    "true",
				"rbac.deckhouse.io/related-with": binding.RelatedWith,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     binding.RoleName,
		},
		Subjects: binding.Subjects,
	}
}
