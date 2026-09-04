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

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	customClusterRoleSnapshots = "custom_cluster_roles"

	// accessLevelKey is both the annotation that marks a custom ClusterRole and the label
	// that the aggregated ClusterRoles (user-authz:<level>:custom) select it by. The
	// annotation is the public contract; the label is maintained by this hook. Nothing else
	// is derived from custom roles anymore: the aggregation happens in Kubernetes and the
	// bindings are reconciled by user-authz-controller.
	accessLevelKey = "user-authz.deckhouse.io/access-level"

	accessLevelUser           = "User"
	accessLevelPrivilegedUser = "PrivilegedUser"
	accessLevelEditor         = "Editor"
	accessLevelAdmin          = "Admin"
	accessLevelClusterEditor  = "ClusterEditor"
	accessLevelClusterAdmin   = "ClusterAdmin"
)

type customClusterRole struct {
	Name string
	// Role is the access level from the annotation; empty when the annotation is absent or
	// invalid (such a role is in the snapshot only to have a stale label removed).
	Role string
	// Label is the current value of the access-level label on the object.
	Label string
}

func applyCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ccr := &customClusterRole{
		Name:  obj.GetName(),
		Label: obj.GetLabels()[accessLevelKey],
	}

	role := obj.GetAnnotations()[accessLevelKey]
	switch role {
	case accessLevelUser, accessLevelPrivilegedUser, accessLevelEditor, accessLevelAdmin, accessLevelClusterEditor, accessLevelClusterAdmin:
		ccr.Role = role
	default:
		if ccr.Label == "" {
			return nil, nil
		}
	}

	return ccr, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("custom_rbac_roles"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       customClusterRoleSnapshots,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyCustomRoleFilter,
		},
	},
}, customClusterRolesHandler)

func customClusterRolesHandler(_ context.Context, input *go_hook.HookInput) error {
	roles, err := snapshotsToCustomClusterRoles(input.Snapshots.Get(customClusterRoleSnapshots))
	if err != nil {
		return fmt.Errorf("failed to convert custom cluster roles snapshots: %w", err)
	}

	syncAccessLevelLabels(input, roles)

	return nil
}

// syncAccessLevelLabels makes the access-level label of every custom ClusterRole equal to its
// annotation, and removes the label from roles that lost the annotation. The label is what the
// aggregated ClusterRoles select by, so a wrong label would grant or revoke rights.
func syncAccessLevelLabels(input *go_hook.HookInput, roles []customClusterRole) {
	for _, role := range roles {
		if role.Label == role.Role {
			continue
		}

		var label interface{}
		if role.Role != "" {
			label = role.Role
		}

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					accessLevelKey: label,
				},
			},
		}

		input.PatchCollector.PatchWithMerge(patch, "rbac.authorization.k8s.io/v1", "ClusterRole", "", role.Name)
	}
}

func snapshotsToCustomClusterRoles(snapshots []pkg.Snapshot) ([]customClusterRole, error) {
	roles := make([]customClusterRole, 0, len(snapshots))

	for role, err := range sdkobjectpatch.SnapshotIter[customClusterRole](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("failed to iterate over '%s' snapshot: %w", customClusterRoleSnapshots, err)
		}

		roles = append(roles, role)
	}

	return roles, nil
}
