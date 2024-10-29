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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/user-authz/handle-manage-bindings",
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
}, syncBindings)

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
	return &filteredUseBinding{
		Name:        binding.Name,
		Namespace:   binding.Namespace,
		RelatedWith: binding.Labels["rbac.deckhouse.io/related-with"],
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
	Name   string                  `json:"name"`
	Labels map[string]string       `json:"aggregationLabels"`
	Rule   *rbacv1.AggregationRule `json:"selectors"`
}

func filterManageRole(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var role rbacv1.ClusterRole
	if err := sdk.FromUnstructured(obj, &role); err != nil {
		return nil, err
	}
	return &filteredManageRole{
		Name:   role.Name,
		Labels: role.Labels,
		Rule:   role.AggregationRule,
	}, nil
}

func syncBindings(input *go_hook.HookInput) error {
	expected := make(map[string]bool)
	for _, snap := range input.Snapshots["manageBindings"] {
		binding := snap.(*filteredManageBinding)
		role, namespaces := roleAndNamespacesByBinding(input.Snapshots["manageRoles"], binding.RoleName)
		useBindingName := fmt.Sprintf("d8:use:binding:%s", binding.Name)
		for namespace := range namespaces {
			input.PatchCollector.Create(createBinding(binding, role, namespace), object_patch.UpdateIfExists())
			expected[useBindingName] = true
		}
	}

	// delete excess use bindings
	for _, snap := range input.Snapshots["useBindings"] {
		existing := snap.(*filteredUseBinding)
		if _, ok := expected[existing.RoleName]; !ok {
			input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", existing.Namespace, existing.Name)
		}
	}

	return nil
}

func roleAndNamespacesByBinding(manageRoles []go_hook.FilterResult, roleName string) (string, map[string]bool) {
	var useRole string
	var found *filteredManageRole
	for _, snap := range manageRoles {
		if role := snap.(*filteredManageRole); role.Name == roleName {
			found = role
			var ok bool
			if useRole, ok = found.Labels["rbac.deckhouse.io/use-role"]; !ok {
				return "", nil
			}
			break
		}
	}
	if found == nil {
		return "", nil
	}

	var namespaces = make(map[string]bool)
	for _, snap := range manageRoles {
		role := snap.(*filteredManageRole)
		if matchAggregationRule(found.Rule, role.Labels) {
			if namespace, ok := role.Labels["rbac.deckhouse.io/namespace"]; ok {
				namespaces[namespace] = true
			}
			if role.Rule != nil {
				for _, nestedSnap := range manageRoles {
					nested := nestedSnap.(*filteredManageRole)
					if matchAggregationRule(role.Rule, nested.Labels) {
						if namespace, ok := nested.Labels["rbac.deckhouse.io/namespace"]; ok {
							namespaces[namespace] = true
						}
					}
				}
			}
		}
	}

	return useRole, namespaces
}

func matchAggregationRule(rule *rbacv1.AggregationRule, roleLabels map[string]string) bool {
	if rule == nil {
		return false
	}
	for _, selector := range rule.ClusterRoleSelectors {
		if selector.MatchLabels != nil {
			if labels.SelectorFromSet(selector.MatchLabels).Matches(labels.Set(roleLabels)) {
				return true
			}
		}
	}
	return false
}

func createBinding(binding *filteredManageBinding, useRoleName string, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			Kind:       "RoleBinding",
			APIVersion: "rbac.authorization.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("d8:use:binding:%s", binding.Name),
			Namespace: namespace,
			Labels: map[string]string{
				"heritage":                       "deckhouse",
				"rbac.deckhouse.io/automated":    "true",
				"rbac.deckhouse.io/related-with": binding.Name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("d8:use:role:%s", useRoleName),
		},
		Subjects: binding.Subjects,
	}
}
