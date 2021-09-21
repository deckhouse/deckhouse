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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/140-user-authz/hooks/internal"
)

const (
	ccrSnapshot = "custom_cluster_roles"
)

type CustomClusterRole struct {
	Name string
	Role string
}

func applyCustomClusterRoleFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ccr := &CustomClusterRole{}

	role := obj.GetAnnotations()["user-authz.deckhouse.io/access-level"]
	switch role {
	case "User", "PrivilegedUser", "Editor", "Admin", "ClusterEditor", "ClusterAdmin":
		ccr.Name = obj.GetName()
		ccr.Role = role
	default:
		return nil, nil
	}
	return ccr, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue(ccrSnapshot),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       ccrSnapshot,
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			FilterFunc: applyCustomClusterRoleFilter,
		},
	},
}, customClusterRolesHandler)

func customClusterRolesHandler(input *go_hook.HookInput) error {
	type internalValuesCustomClusterRoles struct {
		User           []string `json:"user"`
		PrivilegedUser []string `json:"privilegedUser"`
		Editor         []string `json:"editor"`
		Admin          []string `json:"admin"`
		ClusterEditor  []string `json:"clusterEditor"`
		ClusterAdmin   []string `json:"clusterAdmin"`
	}

	var (
		userRoleNames           = set.New()
		privilegedUserRoleNames = set.New()
		editorRoleNames         = set.New()
		adminRoleNames          = set.New()
		clusterEditorRoleNames  = set.New()
		clusterAdminRoleNames   = set.New()
	)

	snapshots := input.Snapshots[ccrSnapshot]

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		customClusterRole := snapshot.(*CustomClusterRole)
		switch customClusterRole.Role {
		case "User":
			userRoleNames.Add(customClusterRole.Name)
			fallthrough
		case "PrivilegedUser":
			privilegedUserRoleNames.Add(customClusterRole.Name)
			fallthrough
		case "Editor":
			editorRoleNames.Add(customClusterRole.Name)
			fallthrough
		case "Admin":
			adminRoleNames.Add(customClusterRole.Name)
			fallthrough
		case "ClusterEditor":
			clusterEditorRoleNames.Add(customClusterRole.Name)
			fallthrough
		case "ClusterAdmin":
			clusterAdminRoleNames.Add(customClusterRole.Name)
		}
	}

	input.Values.Set("userAuthz.internal.customClusterRoles", internalValuesCustomClusterRoles{
		User:           userRoleNames.Slice(),
		PrivilegedUser: privilegedUserRoleNames.Slice(),
		Editor:         editorRoleNames.Slice(),
		Admin:          adminRoleNames.Slice(),
		ClusterEditor:  clusterEditorRoleNames.Slice(),
		ClusterAdmin:   clusterAdminRoleNames.Slice(),
	})

	return nil
}
