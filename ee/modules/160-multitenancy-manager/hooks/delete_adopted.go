/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/multitenancy-manager/delete-adopted",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "roles",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage":                "deckhouse",
					"rbac.deckhouse.io/kind":  "manage",
					"rbac.deckhouse.io/level": "module",
					"module":                  "multitenancy-manager",
				},
			},
			FilterFunc: filterRoles,
		},
		{
			Name:       "bindings",
			ApiVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8:multitenancy-manager:multitenancy-manager"},
			},
			FilterFunc: filterClusterBindings,
		},
	},
}, deleteAdopted)

type filteredRole struct {
	Name string `json:"name"`
}

func filterRoles(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	role := new(rbacv1.ClusterRole)
	if err := sdk.FromUnstructured(obj, role); err != nil {
		return nil, err
	}
	if role.Annotations == nil || len(role.Annotations) == 0 {
		return nil, nil
	}
	if val, ok := role.Annotations["meta.helm.sh/release-namespace"]; ok && val == "d8-multitenancy-manager" {
		return &filteredRole{Name: role.Name}, nil
	}
	return nil, nil
}

type filteredBinding struct {
	Name string `json:"name"`
}

func filterClusterBindings(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	binding := new(rbacv1.ClusterRoleBinding)
	if err := sdk.FromUnstructured(obj, binding); err != nil {
		return nil, err
	}
	if binding.Annotations == nil || len(binding.Annotations) == 0 {
		return nil, nil
	}
	if val, ok := binding.Annotations["meta.helm.sh/release-namespace"]; ok && val == "d8-multitenancy-manager" {
		return &filteredRole{Name: binding.Name}, nil
	}
	return nil, nil
}

func deleteAdopted(input *go_hook.HookInput) error {
	for _, snap := range input.Snapshots["roles"] {
		if snap == nil {
			continue
		}
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRole", "", snap.(*filteredRole).Name)
	}
	for _, snap := range input.Snapshots["bindings"] {
		if snap == nil {
			continue
		}
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", snap.(*filteredBinding).Name)
	}
	return nil
}
