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
		input.PatchCollector.Delete(obj.apiVersion, obj.kind, obj.namespace, obj.name)
	}

	return nil
}
