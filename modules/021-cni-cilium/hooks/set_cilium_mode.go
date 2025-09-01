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

type resultStruct struct {
	DesiredCniConfigSourcePriorityFlagExists bool
	DesiredCniConfigSourcePriority           string
	CniConfigFromSecret                      CiliumConfigStruct
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

	cniConfigSourcePriorityFlagExists := false
	cniConfigSourcePriority := ""
	cniConfigSourcePriority, cniConfigSourcePriorityFlagExists = secret.Annotations[cniConfigSourcePriorityAnnotation]

	return resultStruct{
		DesiredCniConfigSourcePriorityFlagExists: cniConfigSourcePriorityFlagExists,
		DesiredCniConfigSourcePriority:           cniConfigSourcePriority,
		CniConfigFromSecret:                      ciliumConfig,
	}, nil
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
	clusterIsBootstrapped := input.Values.Get("global.clusterIsBootstrapped").Bool()

	cniConfigurationSecrets, err := sdkobjectpatch.UnmarshalToStruct[resultStruct](input.Snapshots, "cni_configuration_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni_configuration_secret snapshot: %w", err)
	}

	cniConfigSourcePriority := "ModuleConfig"
	if len(cniConfigurationSecrets) > 0 {
		if cniConfigurationSecrets[0].DesiredCniConfigSourcePriorityFlagExists {
			if cniConfigurationSecrets[0].DesiredCniConfigSourcePriority != "ModuleConfig" {
				cniConfigSourcePriority = "Secret"
			}
		} else if clusterIsBootstrapped {
			cniConfigSourcePriority = "Secret"
		}
	}
	input.Logger.Info("The priority parameter source for the CNI configuration has been identified", "priority source", cniConfigSourcePriority)

	switch cniConfigSourcePriority {
	case "Secret":
		ciliumConfig := cniConfigurationSecrets[0].CniConfigFromSecret
		if ciliumConfig.Mode != "" {
			input.Values.Set("cniCilium.internal.mode", ciliumConfig.Mode)
		}
		if ciliumConfig.MasqueradeMode != "" {
			input.Values.Set("cniCilium.internal.masqueradeMode", ciliumConfig.MasqueradeMode)
		}
		return nil
	case "ModuleConfig":
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
				// to recover the default value if it was discovered before
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
	}

	// default_mode = Direct
	// default_masqueradeMode = BPF
	return nil
}
