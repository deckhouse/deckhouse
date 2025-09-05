/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

const (
	cniConfigurationSettledKey = "cniConfigurationSettled"
	checkCNIConfigMetricName   = "cniMisconfigured"
	checkCNIConfigMetricGroup  = "d8_check_cni_conf"
	desiredCNIModuleConfigName = "desired-cni-moduleconfig"
	cni                        = "flannel"
	cniName                    = "cni-" + cni
)

type flannelConfigStruct struct {
	PodNetworkMode string `json:"podNetworkMode"`
}

type ciliumConfigStruct struct {
	Mode           string `json:"mode,omitempty"`
	MasqueradeMode string `json:"masqueradeMode,omitempty"`
}

type cniSecretStruct struct {
	CNI     string
	Flannel flannelConfigStruct
	Cilium  ciliumConfigStruct
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 9},
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
			FilterFunc: applyCNIConfigurationFromSecretFilter,
		},
		{
			Name:       "deckhouse_cni_mc",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{cniName},
			},
			FilterFunc: applyCNIMCFilter,
		},
	},
}, checkCni)

func applyCNIConfigurationFromSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// Return nil if
	// error occurred while json parse
	// or d8-cni-configuration secret does not contain key "cni"
	// or value of key "cni" not in [cni-cilium, cni-flannel, cni-simple-bridge]
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}
	cniSecret := cniSecretStruct{}
	cniBytes, ok := secret.Data["cni"]
	if !ok {
		// d8-cni-configuration secret does not contain "cni" field
		return nil, nil
	}
	cniSecret.CNI = string(cniBytes)
	switch cniSecret.CNI {
	case "simple-bridge":
		return cniSecret, nil
	case "flannel":
		flannelConfigJSON, ok := secret.Data["flannel"]
		if ok {
			err = json.Unmarshal(flannelConfigJSON, &cniSecret.Flannel)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal flannel config json: %v", err)
			}
		}
		return cniSecret, nil
	case "cilium":
		ciliumConfigJSON, ok := secret.Data["cilium"]
		if ok {
			err = json.Unmarshal(ciliumConfigJSON, &cniSecret.Cilium)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal cilium config json: %v", err)
			}
		}
		return cniSecret, nil
	default:
		// unknown cni name
		return nil, nil
	}
}

func applyCNIMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &v1alpha1.ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert object to moduleconfig: %v", err)
	}
	if mc.Spec.Enabled == nil || !*mc.Spec.Enabled {
		return nil, nil
	}

	return mc, nil
}

func setCNIMiscMetricAndReq(input *go_hook.HookInput, miss bool) {
	switch miss {
	// misconfigure detected
	case true:
		input.MetricsCollector.Set(checkCNIConfigMetricName, 1,
			map[string]string{
				"cni": cniName,
			}, metrics.WithGroup(checkCNIConfigMetricGroup))
		requirements.SaveValue(cniConfigurationSettledKey, "false")

	// configuration settled
	case false:
		input.MetricsCollector.Set(checkCNIConfigMetricName, 0,
			map[string]string{
				"cni": cniName,
			}, metrics.WithGroup(checkCNIConfigMetricGroup))
		requirements.SaveValue(cniConfigurationSettledKey, "true")
	}
}

func checkCni(_ context.Context, input *go_hook.HookInput) error {
	// Clear a metrics and reqKey
	input.MetricsCollector.Expire(checkCNIConfigMetricGroup)
	requirements.RemoveValue(cniConfigurationSettledKey)
	needUpdateMC := false

	cniSecrets, err := sdkobjectpatch.UnmarshalToStruct[cniSecretStruct](input.Snapshots, "cni_configuration_secret")
	if err != nil {
		setCNIMiscMetricAndReq(input, false)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}
	// Let's check secret.
	// Secret d8-cni-configuration does not exist or exist but contain nil.
	// This means that the current CNI module is enabled and configured via mc, nothing to do.
	if len(cniSecrets) == 0 {
		setCNIMiscMetricAndReq(input, false)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// Secret d8-cni-configuration exist but key "cni" does not equal "flannel".
	// This means that the current CNI module is enabled and configured via mc, nothing to do.
	cniSecret := cniSecrets[0]
	if cniSecret.CNI != cni {
		setCNIMiscMetricAndReq(input, false)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// Secret d8-cni-configuration exist, key "cni" eq "flannel".

	// Prepare desiredCNIModuleConfig
	desiredCNIModuleConfig := &v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleConfig",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cniName,
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled:  ptr.To(true),
			Version:  1,
			Settings: v1alpha1.SettingsValues{},
		},
	}

	deckhouseCniMCs, err := sdkobjectpatch.UnmarshalToStruct[v1alpha1.ModuleConfig](input.Snapshots, "deckhouse_cni_mc")
	if err != nil {
		return fmt.Errorf("failed to unmarshal deckhouse_cni_mc snapshot: %w", err)
	}
	// Let's check what mc exist and explicitly enabled.
	if len(deckhouseCniMCs) == 0 {
		needUpdateMC = true
	} else {
		cniModuleConfig := deckhouseCniMCs[0]
		desiredCNIModuleConfig.Spec.Settings = cniModuleConfig.DeepCopy().Spec.Settings
	}

	// Skip comparison if in secret d8-cni-configuration key "flannel" does not exist or empty.
	if cniSecret.Flannel != (flannelConfigStruct{}) {
		// Secret d8-cni-configuration exist, key "cni" eq "flannel" and key "flannel" does not empty.
		// Let's compare secret with module configuration.
		switch cniSecret.Flannel.PodNetworkMode {
		case "host-gw":
			value, ok := input.ConfigValues.GetOk("cniFlannel.podNetworkMode")
			if !ok || value.String() != "HostGW" {
				desiredCNIModuleConfig.Spec.Settings["podNetworkMode"] = "HostGW"
				needUpdateMC = true
			}
		case "vxlan":
			value, ok := input.ConfigValues.GetOk("cniFlannel.podNetworkMode")
			if !ok || value.String() != "VXLAN" {
				desiredCNIModuleConfig.Spec.Settings["podNetworkMode"] = "VXLAN"
				needUpdateMC = true
			}
		case "":
			value, ok := input.ConfigValues.GetOk("cniFlannel.podNetworkMode")
			if !ok || value.String() != "HostGW" {
				desiredCNIModuleConfig.Spec.Settings["podNetworkMode"] = "HostGW"
				needUpdateMC = true
			}
		default:
			setCNIMiscMetricAndReq(input, true)
			input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
			return fmt.Errorf("unknown flannel podNetworkMode %s", cniSecret.Flannel.PodNetworkMode)
		}
	}

	if needUpdateMC {
		desiredCNIModuleConfigYAML, err := yaml.Marshal(*desiredCNIModuleConfig)
		if err != nil {
			return fmt.Errorf("cannot marshal desired CNI moduleConfig, err: %w", err)
		}
		data := map[string]string{cniName + "-mc.yaml": string(desiredCNIModuleConfigYAML)}
		cm := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      desiredCNIModuleConfigName,
				Namespace: "d8-system",
			},
			Data: data,
		}
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		input.PatchCollector.CreateOrUpdate(cm)
		setCNIMiscMetricAndReq(input, true)
		return nil
	}

	// All configuration settled, nothing to do.
	setCNIMiscMetricAndReq(input, false)
	input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
	return nil
}
