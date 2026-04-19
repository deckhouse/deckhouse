/*
Copyright 2023 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "vm-cidrs",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"virtualization"},
			},
			FilterFunc:          applyVMCIDRsFilter,
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, applyVMCIDRs)

func applyVMCIDRsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert virtualization moduleconfig: %v", err)
	}

	settings := mc.Spec.Settings.GetMap()
	if len(settings) == 0 {
		return nil, nil
	}

	return settings["virtualMachineCIDRs"], nil
}

func applyVMCIDRs(_ context.Context, input *go_hook.HookInput) error {
	snaps, err := sdkobjectpatch.UnmarshalToStruct[[]any](input.Snapshots, "vm-cidrs")
	if err != nil {
		return fmt.Errorf("failed to unmarshal vm-cidrs snapshot: %w", err)
	}
	if len(snaps) == 1 {
		input.Values.Set("cniCilium.internal.vmCIDRs", snaps[0])
	}
	return nil
}
