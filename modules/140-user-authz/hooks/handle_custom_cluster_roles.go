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

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	customClusterRoleSnapshots = "custom_cluster_roles"

	accessLevelUser           = "User"
	accessLevelPrivilegedUser = "PrivilegedUser"
	accessLevelEditor         = "Editor"
	accessLevelAdmin          = "Admin"
	accessLevelClusterEditor  = "ClusterEditor"
	accessLevelClusterAdmin   = "ClusterAdmin"
)

type customClusterRole struct {
	Name string
	Role string
}

func applyCustomRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ccr := &customClusterRole{
		Name: obj.GetName(),
	}

	role := obj.GetAnnotations()["user-authz.deckhouse.io/access-level"]
	switch role {
	case accessLevelUser, accessLevelPrivilegedUser, accessLevelEditor, accessLevelAdmin, accessLevelClusterEditor, accessLevelClusterAdmin:
		ccr.Role = role
	default:
		return nil, nil
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
	customClusterRoles, err := snapshotsToInternalValuesCustomClusterRoles(input.Snapshots.Get(customClusterRoleSnapshots))
	if err != nil {
		return fmt.Errorf("failed to convert custom cluster roles snapshots: %w", err)
	}

	input.Values.Set("userAuthz.internal.customClusterRoles", customClusterRoles)
	return nil
}

type internalValuesCustomClusterRoles struct {
	User           []string `json:"user"`
	PrivilegedUser []string `json:"privilegedUser"`
	Editor         []string `json:"editor"`
	Admin          []string `json:"admin"`
	ClusterEditor  []string `json:"clusterEditor"`
	ClusterAdmin   []string `json:"clusterAdmin"`
}

func snapshotsToInternalValuesCustomClusterRoles(snapshots []pkg.Snapshot) (internalValuesCustomClusterRoles, error) {
	var (
		userRoleNames           = set.New()
		privilegedUserRoleNames = set.New()
		editorRoleNames         = set.New()
		adminRoleNames          = set.New()
		clusterEditorRoleNames  = set.New()
		clusterAdminRoleNames   = set.New()
	)

	for customRole, err := range sdkobjectpatch.SnapshotIter[customClusterRole](snapshots) {
		if err != nil {
			return internalValuesCustomClusterRoles{}, fmt.Errorf("failed to iterate over '%s' snapshot: %w", customClusterRoleSnapshots, err)
		}

		switch customRole.Role {
		case accessLevelUser:
			userRoleNames.Add(customRole.Name)
			fallthrough
		case accessLevelPrivilegedUser:
			privilegedUserRoleNames.Add(customRole.Name)
			fallthrough
		case accessLevelEditor:
			editorRoleNames.Add(customRole.Name)
			fallthrough
		case accessLevelAdmin:
			adminRoleNames.Add(customRole.Name)
			fallthrough
		case accessLevelClusterEditor:

			clusterEditorRoleNames.Add(customRole.Name)
			fallthrough
		case accessLevelClusterAdmin:
			clusterAdminRoleNames.Add(customRole.Name)
		}
	}
	values := internalValuesCustomClusterRoles{
		User:           userRoleNames.Slice(),
		PrivilegedUser: privilegedUserRoleNames.Slice(),
		Editor:         editorRoleNames.Slice(),
		Admin:          adminRoleNames.Slice(),
		ClusterEditor:  clusterEditorRoleNames.Slice(),
		ClusterAdmin:   clusterAdminRoleNames.Slice(),
	}

	return values, nil
}
