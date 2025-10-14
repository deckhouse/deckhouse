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

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
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

func applyCNIFromMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to moduleconfig: %v", err)
	}

	return mc, nil
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
		{
			Name:       "deckhouse_cni_mc",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{cniName},
			},
			FilterFunc: applyCNIFromMCFilter,
		},
	},
}, setCiliumMode)

func setCiliumMode(_ context.Context, input *go_hook.HookInput) error {
	clusterIsBootstrapped := input.Values.Get("global.clusterIsBootstrapped").Bool()

	cniConfigurationSecrets, err := sdkobjectpatch.UnmarshalToStruct[resultStruct](input.Snapshots, "cni_configuration_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni_configuration_secret snapshot: %w", err)
	}

	cniModuleConfigs, err := sdkobjectpatch.UnmarshalToStruct[v1alpha1.ModuleConfig](input.Snapshots, "deckhouse_cni_mc")
	if err != nil {
		return fmt.Errorf("failed to unmarshal deckhouse_cni_mc snapshot: %w", err)
	}

	typeOfMergingCNIParameters := "SecretNotExists"

	if len(cniConfigurationSecrets) > 0 {
		switch {
		case len(cniModuleConfigs) == 0:
			typeOfMergingCNIParameters = "SecretExistsAndHasPriority"
		case cniConfigurationSecrets[0].DesiredCniConfigSourcePriorityFlagExists:
			if cniConfigurationSecrets[0].DesiredCniConfigSourcePriority == "ModuleConfig" {
				typeOfMergingCNIParameters = "SecretExistsAndMCHasPriority"
			} else {
				typeOfMergingCNIParameters = "SecretExistsAndHasPriority"
			}
		case clusterIsBootstrapped:
			typeOfMergingCNIParameters = "SecretExistsAndHasPriority"
		default:
			typeOfMergingCNIParameters = "SecretExistsAndMCHasPriority"
		}
	}
	input.Logger.Debug("The type of CNI parameter merging has been identified.", "merging type is ", typeOfMergingCNIParameters)

	switch typeOfMergingCNIParameters {
	case "SecretExistsAndHasPriority":
		// Secret exists and has priority (old logic); merging priority: Secret > MC(if exists) > Defaults

		// masqueradeMode
		if value := cniConfigurationSecrets[0].CniConfigFromSecret.MasqueradeMode; value != "" {
			input.Values.Set("cniCilium.internal.masqueradeMode", value)
		}
		// tunnelMode
		if value := cniConfigurationSecrets[0].CniConfigFromSecret.Mode; value != "" {
			input.Values.Set("cniCilium.internal.mode", value)
		}

		return nil
	case "SecretExistsAndMCHasPriority":
		// Secret and MC exist, and MC has priority (new logic); merging priority: MC > Secret > Default

		// masqueradeMode
		if value, ok := cniModuleConfigs[0].Spec.Settings["masqueradeMode"]; ok && value != nil {
			input.Values.Set("cniCilium.internal.masqueradeMode", value.(string))
		} else if value := cniConfigurationSecrets[0].CniConfigFromSecret.MasqueradeMode; value != "" {
			input.Values.Set("cniCilium.internal.masqueradeMode", value)
		} else if value, ok := input.ConfigValues.GetOk("cniCilium.masqueradeMode"); ok {
			input.Values.Set("cniCilium.internal.masqueradeMode", value.String())
		}
		// tunnelMode
		if value, ok := cniModuleConfigs[0].Spec.Settings["tunnelMode"]; ok && value != nil {
			switch value.(string) {
			case "VXLAN":
				input.Values.Set("cniCilium.internal.mode", "VXLAN")
				return nil
			case "Disabled":
				input.Values.Set("cniCilium.internal.mode", "Direct")
			}
		} else if value := cniConfigurationSecrets[0].CniConfigFromSecret.Mode; value != "" {
			input.Values.Set("cniCilium.internal.mode", value)
		} else if value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode"); ok {
			switch value.String() {
			case "VXLAN":
				input.Values.Set("cniCilium.internal.mode", "VXLAN")
				return nil
			case "Disabled":
				input.Values.Set("cniCilium.internal.mode", "Direct")
			}
		}
		// createNodeRoutes
		if value, ok := cniModuleConfigs[0].Spec.Settings["createNodeRoutes"]; ok && value != nil {
			if value.(bool) {
				input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
			}
		} else if value := cniConfigurationSecrets[0].CniConfigFromSecret.Mode; value == "DirectWithNodeRoutes" {
			input.Values.Set("cniCilium.internal.mode", value)
		} else if value, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes"); ok && value.Bool() {
			input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
		}

		// for static clusters we should use DirectWithNodeRoutes mode
		if value, ok := input.Values.GetOk("global.clusterConfiguration.clusterType"); ok && value.String() == "Static" {
			input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
		}

		return nil
	default:
		// No secret exists (default logic); merging priority: MC(if exists) > Defaults

		// masqueradeMode
		if value, ok := input.ConfigValues.GetOk("cniCilium.masqueradeMode"); ok {
			input.Values.Set("cniCilium.internal.masqueradeMode", value.String())
		}
		// tunnelMode
		if value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode"); ok {
			switch value.String() {
			case "VXLAN":
				input.Values.Set("cniCilium.internal.mode", "VXLAN")
				return nil
			case "Disabled":
				// to recover the default value if it was discovered before
				input.Values.Set("cniCilium.internal.mode", "Direct")
			}
		}
		// createNodeRoutes
		if value, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes"); ok && value.Bool() {
			input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
		}

		// for static clusters we should use DirectWithNodeRoutes mode
		if value, ok := input.Values.GetOk("global.clusterConfiguration.clusterType"); ok && value.String() == "Static" {
			input.Values.Set("cniCilium.internal.mode", "DirectWithNodeRoutes")
		}

		return nil
	}

	// default_mode = Direct
	// default_masqueradeMode = BPF
	// return nil
}
