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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const hasVirtualControlPlanePath = "controlPlaneManager.internal.hasVirtualControlPlane"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "virtual_control_planes",
			ApiVersion:                   "control-plane.deckhouse.io/v1alpha1",
			Kind:                         "VirtualControlPlane",
			WaitForSynchronization:       ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			FilterFunc:                   applyVirtualControlPlaneFilter,
		},
	},
}, handleVirtualControlPlane)

func applyVirtualControlPlaneFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func handleVirtualControlPlane(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set(hasVirtualControlPlanePath, len(input.Snapshots.Get("virtual_control_planes")) > 0)
	return nil
}
