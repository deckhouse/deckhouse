package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
	Queue:     "/modules/multitenancy-manager/delete-adopted-roles",
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
	},
}, deleteAdoptedRoles)

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
	if val, ok := role.Annotations["meta.helm.sh/release-namespace"]; ok || val == "d8-multitenancy-manager" {
		return &filteredRole{Name: role.Name}, nil
	}
	if val, ok := role.Annotations["meta.helm.sh/release-name"]; ok && val == "d8:manage:capability:module:multitenancy-manager:edit" {
		return &filteredRole{Name: role.Name}, nil
	}
	return nil, nil
}

func deleteAdoptedRoles(input *go_hook.HookInput) error {
	for _, snap := range input.Snapshots["roles"] {
		if snap == nil {
			continue
		}
		input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRole", "", snap.(*filteredRole).Name)
	}
	return nil
}
