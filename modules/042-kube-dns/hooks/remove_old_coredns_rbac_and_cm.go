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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 15},
}, removeKubeDNSRBACAndConfigMap)

func removeKubeDNSRBACAndConfigMap(_ context.Context, input *go_hook.HookInput) error {
	input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRole", "", "system:coredns")
	input.PatchCollector.Delete("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "", "system:coredns")
	input.PatchCollector.Delete("v1", "ServiceAccount", "kube-system", "coredns")
	input.PatchCollector.Delete("v1", "ConfigMap", "kube-system", "coredns")
	return nil
}
