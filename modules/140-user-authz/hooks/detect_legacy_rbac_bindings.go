/*
Copyright 2026 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Detects ClusterRoleBindings/RoleBindings referring to legacy experimental RBAC v2 roles
// (d8:use:* and d8:manage:*) which were renamed to d8:namespace:*/d8:system:*/d8:subsystem:*.
// The result is stored as a release requirement value: the DKP upgrade is blocked until
// such bindings are removed or rebound (see modules/140-user-authz/requirements/check.go).

const (
	legacyRBACBindingsCountKey = "userAuthz:legacyRBACBindingsCount"
	legacyRBACBindingsListKey  = "userAuthz:legacyRBACBindingsList"

	legacyRBACBindingsListLimit = 10
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authz/detect-legacy-rbac-bindings",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "legacyClusterRoleBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: filterLegacyClusterRoleBinding,
		},
		{
			Name:       "legacyRoleBindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			FilterFunc: filterLegacyRoleBinding,
		},
	},
}, detectLegacyRBACBindings)

type filteredLegacyBinding struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

func isLegacyRoleRef(roleRef rbacv1.RoleRef, objLabels map[string]string) bool {
	if roleRef.Kind != "ClusterRole" {
		return false
	}
	// Skip bindings managed by Deckhouse itself (automated bindings are recreated by hooks).
	if objLabels["rbac.deckhouse.io/automated"] == "true" || objLabels["heritage"] == "deckhouse" {
		return false
	}
	return strings.HasPrefix(roleRef.Name, "d8:use:") || strings.HasPrefix(roleRef.Name, "d8:manage:")
}

func filterLegacyClusterRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var binding rbacv1.ClusterRoleBinding
	if err := sdk.FromUnstructured(obj, &binding); err != nil {
		return nil, err
	}
	if !isLegacyRoleRef(binding.RoleRef, binding.Labels) {
		return nil, nil
	}
	return &filteredLegacyBinding{Kind: "ClusterRoleBinding", Name: binding.Name}, nil
}

func filterLegacyRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var binding rbacv1.RoleBinding
	if err := sdk.FromUnstructured(obj, &binding); err != nil {
		return nil, err
	}
	if !isLegacyRoleRef(binding.RoleRef, binding.Labels) {
		return nil, nil
	}
	return &filteredLegacyBinding{Kind: "RoleBinding", Name: binding.Name, Namespace: binding.Namespace}, nil
}

func detectLegacyRBACBindings(_ context.Context, input *go_hook.HookInput) error {
	var found []string
	for _, snapshot := range []string{"legacyClusterRoleBindings", "legacyRoleBindings"} {
		for binding, err := range sdkobjectpatch.SnapshotIter[filteredLegacyBinding](input.Snapshots.Get(snapshot)) {
			if err != nil {
				return fmt.Errorf("failed to iterate over '%s' snapshot: %w", snapshot, err)
			}
			if binding.Name == "" {
				continue
			}
			if binding.Namespace != "" {
				found = append(found, fmt.Sprintf("%s/%s/%s", binding.Kind, binding.Namespace, binding.Name))
			} else {
				found = append(found, fmt.Sprintf("%s/%s", binding.Kind, binding.Name))
			}
		}
	}

	if len(found) == 0 {
		requirements.RemoveValue(legacyRBACBindingsCountKey)
		requirements.RemoveValue(legacyRBACBindingsListKey)
		return nil
	}

	list := found
	if len(list) > legacyRBACBindingsListLimit {
		list = list[:legacyRBACBindingsListLimit]
	}

	requirements.SaveValue(legacyRBACBindingsCountKey, len(found))
	requirements.SaveValue(legacyRBACBindingsListKey, strings.Join(list, ", "))

	return nil
}
