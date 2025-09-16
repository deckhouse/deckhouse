// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

/*
This hook enables cni module enabled either explicitly in configuration or
during installation in Secret/d8-cni-configuration.

Developer notes:
- It uses "dynamic enable" feature of addon-operator to enable module in runtime.
- It executes on Synchronization to return values patch before ConvergeModules task.
- It is the only hook that subscribes to configuration ConfigMap because
  there is no way to get enabled modules list in global hook.
*/

const (
	cniConfigurationSettledKey = "cniConfigurationSettled"
)

var (
	cniNameToModule = map[string]string{
		"flannel":       "cniFlannelEnabled",
		"simple-bridge": "cniSimpleBridgeEnabled",
		"cilium":        "cniCiliumEnabled",
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cni_name",
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cni-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: applyCniConfigFilter,
		},
		{
			Name:       "deckhouse_mc",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cni-flannel", "cni-cilium", "cni-simple-bridge"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   applyMCFilter,
		},
	},
}, enableCni)

func applyMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	v, _, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "enabled")
	if err != nil {
		return nil, err
	}

	if !v {
		return nil, nil
	}

	return obj.GetName(), nil
}

func applyCniConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.Secret
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	cni, ok := cm.Data["cni"]
	if ok {
		return string(cni), nil
	}

	return nil, nil
}

func enableCni(_ context.Context, input *go_hook.HookInput) error {
	requirements.RemoveValue(cniConfigurationSettledKey)

	cniNameSnap, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "cni_name")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni_name snapshot: %w", err)
	}

	deckhouseMCSnap := input.Snapshots.Get("deckhouse_mc")
	explicitlyEnabledCNIs := set.NewFromSnapshot(deckhouseMCSnap)

	if len(cniNameSnap) == 0 {
		input.Logger.Warn("CNI name not found")
		return nil
	}

	if len(explicitlyEnabledCNIs) > 1 {
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		return fmt.Errorf("more than one CNI enabled: %v", explicitlyEnabledCNIs.Slice())
	} else if len(explicitlyEnabledCNIs) == 1 {
		input.Logger.Info("enabled CNI from Deckhouse ModuleConfig", slog.String("cni", explicitlyEnabledCNIs.Slice()[0]))
		return nil
	}
	// nor any CNI enabled directly via MC, found default CNI from secret
	cniToEnable := cniNameSnap[0]
	if _, ok := cniNameToModule[cniToEnable]; !ok {
		input.Logger.Warn("Incorrect cni name. Skip", slog.String("cni", cniToEnable))
		return nil
	}

	input.Logger.Info("enabled CNI by secret", slog.String("cni", cniToEnable))
	input.Values.Set(cniNameToModule[cniToEnable], true)
	return nil
}
