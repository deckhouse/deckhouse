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

// Emits a metric for every (Cluster)RoleBinding whose roleRef still points at a DEPRECATED RBACv2
// role name (d8:manage:* / d8:use:role:*). The RBACv2 role model was renamed; the old names are kept
// alive for one release by the aliases in templates/rbacv2-compat/. This metric drives the
// D8UserAuthzDeprecatedRBACv2RoleInUse alert that nudges operators to migrate their bindings to the
// new names (d8:{system,subsystem,namespace,project}:*) before the aliases are removed next release.

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const deprecatedRBACv2Metric = "d8_rbacv2_deprecated_role_in_use"

// deprecatedRoleNamePrefixes are the legacy RBACv2 role-name prefixes replaced by the new model.
// A ClusterRole roleRef starting with any of these is served (for one release) by a compat alias.
var deprecatedRoleNamePrefixes = []string{
	"d8:manage:",
	"d8:use:role:",
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/user-authz/deprecated-rbacv2-bindings",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "deprecated_clusterrolebindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			FilterFunc: filterDeprecatedClusterRoleBinding,
		},
		{
			Name:       "deprecated_rolebindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
			FilterFunc: filterDeprecatedRoleBinding,
		},
	},
}, handleDeprecatedRBACv2Bindings)

// deprecatedBinding is the minimal projection of a binding that still references a deprecated role.
// The FilterFunc returns nil for every other binding, so the snapshot only holds the offenders — the
// hook never keeps the full set of cluster bindings in memory.
type deprecatedBinding struct {
	BindingKind string `json:"binding_kind"`
	BindingName string `json:"binding_name"`
	Namespace   string `json:"namespace"`
	RoleName    string `json:"role_name"`
}

func deprecatedRoleName(name string) bool {
	for _, prefix := range deprecatedRoleNamePrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func filterDeprecatedClusterRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.ClusterRoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if binding.RoleRef.Kind != "ClusterRole" || !deprecatedRoleName(binding.RoleRef.Name) {
		return nil, nil
	}
	return &deprecatedBinding{
		BindingKind: "ClusterRoleBinding",
		BindingName: binding.Name,
		RoleName:    binding.RoleRef.Name,
	}, nil
}

func filterDeprecatedRoleBinding(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.RoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if binding.RoleRef.Kind != "ClusterRole" || !deprecatedRoleName(binding.RoleRef.Name) {
		return nil, nil
	}
	return &deprecatedBinding{
		BindingKind: "RoleBinding",
		BindingName: binding.Name,
		Namespace:   binding.Namespace,
		RoleName:    binding.RoleRef.Name,
	}, nil
}

func handleDeprecatedRBACv2Bindings(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(deprecatedRBACv2Metric)

	for _, snapshotName := range []string{"deprecated_clusterrolebindings", "deprecated_rolebindings"} {
		for binding, err := range sdkobjectpatch.SnapshotIter[deprecatedBinding](input.Snapshots.Get(snapshotName)) {
			if err != nil {
				return fmt.Errorf("failed to iterate over '%s' snapshot: %w", snapshotName, err)
			}
			input.MetricsCollector.Set(
				deprecatedRBACv2Metric, 1,
				map[string]string{
					"binding_kind": binding.BindingKind,
					"binding_name": binding.BindingName,
					"namespace":    binding.Namespace,
					"role_name":    binding.RoleName,
				},
				metrics.WithGroup(deprecatedRBACv2Metric),
			)
		}
	}
	return nil
}
