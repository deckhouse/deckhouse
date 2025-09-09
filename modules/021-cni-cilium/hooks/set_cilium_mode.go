/*
Copyright 2022 Flant JSC

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
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

type CiliumConfigStruct struct {
	Mode           string `json:"mode,omitempty"`
	MasqueradeMode string `json:"masqueradeMode,omitempty"`
}

func applyCNIConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}

	if string(secret.Data["cni"]) != "cilium" {
		return nil, nil
	}

	var ciliumConfig CiliumConfigStruct
	ciliumConfigJSON, ok := secret.Data["cilium"]
	if !ok {
		return nil, nil
	}

	err = json.Unmarshal(ciliumConfigJSON, &ciliumConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal cilium config json: %v", err)
	}
	return ciliumConfig, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cni_configuration_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cni-configuration"},
			},
			FilterFunc: applyCNIConfigurationSecretFilter,
		},
	},
}, setCiliumMode)

func setCiliumMode(_ context.Context, input *go_hook.HookInput) error {
	// if secret exists, use it
	cniConfigurationSecrets, err := sdkobjectpatch.UnmarshalToStruct[CiliumConfigStruct](input.Snapshots, "cni_configuration_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni_configuration_secret snapshot: %w", err)
	}

	if len(cniConfigurationSecrets) > 0 {
		ciliumConfig := cniConfigurationSecrets[0]
		if ciliumConfig.Mode != "" {
			input.Values.Set("cniCilium.internal.mode", ciliumConfig.Mode)
		}
		if ciliumConfig.MasqueradeMode != "" {
			input.Values.Set("cniCilium.internal.masqueradeMode", ciliumConfig.MasqueradeMode)
		}
		return nil
	}

	value, ok := input.ConfigValues.GetOk("cniCilium.masqueradeMode")
	if ok {
		input.Values.Set("cniCilium.internal.masqueradeMode", value.String())
	}

	value, ok = input.ConfigValues.GetOk("cniCilium.tunnelMode")
	if ok {
		switch value.String() {
		case "VXLAN":
			input.Values.Set("cniCilium.internal.mode", "VXLAN")
			return nil
		case "Disabled":
			// to recover default value if it was discovered before
			input.Values.Set("cniCilium.internal.mode", "Direct")
		}
	}

	value, ok = input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok && value.Bool() {
		input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
	}

	// for static clusters we should use DirectWithNodeRoutes mode
	value, ok = input.Values.GetOk("global.clusterConfiguration.clusterType")
	if ok && value.String() == "Static" {
		input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
	}

	// default
	// mode = Direct
	// masqueradeMode = BPF
	return nil
}
