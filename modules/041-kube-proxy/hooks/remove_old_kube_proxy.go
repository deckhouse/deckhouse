package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, removeOldKubeProxyResourcesHandler)

type removeObject struct {
	apiVersion string
	kind       string
	namespace  string
	name       string
}

func removeOldKubeProxyResourcesHandler(input *go_hook.HookInput) error {
	objects := []removeObject{
		{
			apiVersion: "rbac.authorization.k8s.io/v1",
			kind:       "ClusterRoleBinding",
			namespace:  "",
			name:       "kubeadm:node-proxier",
		},
		{
			apiVersion: "rbac.authorization.k8s.io/v1",
			kind:       "Role",
			namespace:  "kube-system",
			name:       "kube-proxy",
		},
		{
			apiVersion: "rbac.authorization.k8s.io/v1",
			kind:       "RoleBinding",
			namespace:  "kube-system",
			name:       "kube-proxy",
		},
		{
			apiVersion: "v1",
			kind:       "ServiceAccount",
			namespace:  "kube-system",
			name:       "kube-proxy",
		},
		{
			apiVersion: "v1",
			kind:       "ConfigMap",
			namespace:  "kube-system",
			name:       "kube-proxy",
		},
		{
			apiVersion: "apps/v1",
			kind:       "DaemonSet",
			namespace:  "kube-system",
			name:       "kube-proxy",
		},
	}

	for _, obj := range objects {
		err := input.ObjectPatcher.DeleteObject(obj.apiVersion, obj.kind, obj.namespace, obj.kind, "")
		if err != nil {
			return err
		}
	}

	return nil
}
