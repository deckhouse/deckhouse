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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "rbac.deckhouse.io/scope",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"system", "subsystem"},
					},
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
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterAutomaticUseBinding,
		},
	},
}, syncBindings)

type filteredUseBinding struct {
	Name      string           `json:"name"`
	Namespace string           `json:"namespace"`
	RoleName  string           `json:"role_name"`
	Subjects  []rbacv1.Subject `json:"subjects"`
}

func filterAutomaticUseBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var binding rbacv1.RoleBinding
	if err := sdk.FromUnstructured(obj, &binding); err != nil {
		return nil, err
	}
	return &filteredUseBinding{
		Name:      binding.Name,
		Namespace: binding.Namespace,
		RoleName:  binding.RoleRef.Name,
		Subjects:  binding.Subjects,
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

func syncBindings(_ context.Context, input *go_hook.HookInput) error {
	// Materialize and index the manage roles once. Re-decoding the snapshot and
	// recompiling aggregation selectors for every manage binding would be
	// needlessly quadratic.
	roles, err := indexManageRoles(input.Snapshots.Get("manageRoles"))
	if err != nil {
		return err
	}

	expected := make(map[string]bool)
	for binding, err := range sdkobjectpatch.SnapshotIter[filteredManageBinding](input.Snapshots.Get("manageBindings")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'manageBindings' snapshot: %w", err)
		}
		useRole, namespaces := roleAndNamespacesByBinding(roles, binding.RoleName)

		useBindingName := fmt.Sprintf("d8:namespace:%s:binding:%s", useRole, binding.Name)
		for namespace := range namespaces {
			input.PatchCollector.CreateOrUpdate(createBinding(&binding, useRole, namespace))
			// Track by (namespace, name): one manage binding fans out to the
			// same RoleBinding name across many namespaces, so keying by name
			// alone would keep an orphan alive whenever a namespace drops out
			// of the binding while any other namespace remains.
			expected[useBindingKey(namespace, useBindingName)] = true
		}
	}

	// delete excess use bindings
	for existing, err := range sdkobjectpatch.SnapshotIter[filteredUseBinding](input.Snapshots.Get("useBindings")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'useBindings' snapshot: %w", err)
		}
		if _, ok := expected[useBindingKey(existing.Namespace, existing.Name)]; !ok {
			input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "RoleBinding", existing.Namespace, existing.Name)
		}
	}

	return nil
}

// useBindingKey identifies an automated use RoleBinding uniquely. A single
// manage binding produces RoleBindings with the same name in every target
// namespace, so reconciliation must compare by namespace and name together.
func useBindingKey(namespace, name string) string {
	return namespace + "/" + name
}

// indexedManageRole is the decoded form of a manage ClusterRole with its
// aggregation selectors compiled once, so the aggregation walk can match
// children without rebuilding selectors on every comparison.
type indexedManageRole struct {
	name      string
	labels    map[string]string
	selectors []labels.Selector
}

// indexManageRoles materializes the manage roles snapshot and precompiles each
// role's aggregation selectors. Using metav1.LabelSelectorAsSelector honors both
// matchLabels and matchExpressions; the previous matchLabels-only matching
// silently skipped expression-based aggregation rules.
func indexManageRoles(manageRoles []pkg.Snapshot) ([]indexedManageRole, error) {
	roles := make([]indexedManageRole, 0, len(manageRoles))
	for role, err := range sdkobjectpatch.SnapshotIter[filteredManageRole](manageRoles) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over 'manageRoles' snapshot: %w", err)
		}

		indexed := indexedManageRole{
			name:   role.Name,
			labels: role.Labels,
		}
		if role.Rule != nil {
			for i := range role.Rule.ClusterRoleSelectors {
				selector, err := metav1.LabelSelectorAsSelector(&role.Rule.ClusterRoleSelectors[i])
				if err != nil {
					return nil, fmt.Errorf("invalid aggregation selector on manage role '%s': %w", role.Name, err)
				}
				indexed.selectors = append(indexed.selectors, selector)
			}
		}
		roles = append(roles, indexed)
	}
	return roles, nil
}

// roleAndNamespacesByBinding resolves the use-role and the set of namespaces the
// given manage role grants by walking the aggregation graph. It returns an empty
// use-role and no namespaces when the bound role is unknown or carries no
// rbac.deckhouse.io/use-role label.
func roleAndNamespacesByBinding(roles []indexedManageRole, roleName string) (string, map[string]bool) {
	var found *indexedManageRole
	for i := range roles {
		if roles[i].name == roleName {
			found = &roles[i]
			break
		}
	}
	if found == nil {
		return "", nil
	}

	useRole, ok := found.labels["rbac.deckhouse.io/use-role"]
	if !ok {
		return "", nil
	}

	// Walk the whole aggregation chain (bound role -> roles/capabilities it
	// aggregates -> ... -> leaf capabilities) collecting every
	// rbac.deckhouse.io/namespace label reachable from the bound role. A
	// fixed-depth descent misses tiers that sit more than one level above the
	// namespaced capabilities, e.g. d8:system:superadmin ->
	// d8:subsystem:<s>:superadmin -> d8:subsystem:<s>:manager -> capability.
	namespaces := make(map[string]bool)
	visited := map[string]bool{found.name: true}
	collectManageNamespaces(found, roles, namespaces, visited)

	return useRole, namespaces
}

// collectManageNamespaces descends into every manage role/capability whose labels
// match node's aggregation selectors, recording the rbac.deckhouse.io/namespace
// label of each reached object. The visited set guards against cycles and
// repeated work.
func collectManageNamespaces(node *indexedManageRole, roles []indexedManageRole, namespaces, visited map[string]bool) {
	if len(node.selectors) == 0 {
		return
	}
	for i := range roles {
		child := &roles[i]
		if !matchAggregationSelectors(node.selectors, child.labels) {
			continue
		}
		if namespace, ok := child.labels["rbac.deckhouse.io/namespace"]; ok {
			namespaces[namespace] = true
		}
		if visited[child.name] {
			continue
		}
		visited[child.name] = true
		collectManageNamespaces(child, roles, namespaces, visited)
	}
}

// matchAggregationSelectors reports whether any of the precompiled aggregation
// selectors matches the candidate role's labels.
func matchAggregationSelectors(selectors []labels.Selector, roleLabels map[string]string) bool {
	set := labels.Set(roleLabels)
	for _, selector := range selectors {
		if selector.Matches(set) {
			return true
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
			Name:      fmt.Sprintf("d8:namespace:%s:binding:%s", useRoleName, binding.Name),
			Namespace: namespace,
			Annotations: map[string]string{
				"rbac.deckhouse.io/related-with": binding.Name,
			},
			Labels: map[string]string{
				"heritage":                    "deckhouse",
				"rbac.deckhouse.io/automated": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("d8:namespace:%s", useRoleName),
		},
		Subjects: binding.Subjects,
	}
}
