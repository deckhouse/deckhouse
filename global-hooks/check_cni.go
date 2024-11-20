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
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	cniConfigurationSettledKey = "cniConfigurationSettled"
)

type FlannelConfig struct {
	PodNetworkMode string `json:"podNetworkMode"`
}

type CiliumConfigStruct struct {
	Mode           string `json:"mode,omitempty"`
	MasqueradeMode string `json:"masqueradeMode,omitempty"`
}

type cniSecretStruct struct {
	cni     string
	flannel FlannelConfig
	cilium  CiliumConfigStruct
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
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
				MatchNames: []string{"cni-flannel", "cni-cilium", "cni-simple-bridge"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   applyCNIMCFilter,
		},
	},
}, checkCni)

func applyCNIConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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
	cniSecret.cni = string(cniBytes)
	switch cniSecret.cni {
	case "simple-bridge":
		return cniSecret, nil
	case "flannel":
		flannelConfigJSON, ok := secret.Data["flannel"]
		if ok {
			err = json.Unmarshal(flannelConfigJSON, &cniSecret.flannel)
			if err != nil {
				return nil, fmt.Errorf("cannot unmarshal flannel config json: %v", err)
			}
		}
		return cniSecret, nil
	case "cilium":
		ciliumConfigJSON, ok := secret.Data["cilium"]
		if ok {
			err = json.Unmarshal(ciliumConfigJSON, &cniSecret.cilium)
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
func checkCni(input *go_hook.HookInput) error {
	// Clear a metrics and reqKey
	input.MetricsCollector.Expire("enabledCNI")
	input.MetricsCollector.Expire("CNIMisconfigured")
	// requirements.SaveValue(cniConfigurationSettledKey, "false")
	requirements.RemoveValue(cniConfigurationSettledKey)

	// Secret d8-cni-configuration does not exist
	// or exist but key cni does not exist
	//	or value is empty
	//		or not in [cni-cilium, cni-flannel, cni-simple-bridge].
	//cniSecret := cniSecretStruct{}
	// get cniSecret
	cniSecret := cniSecretStruct{}
	if len(input.Snapshots["cni_configuration_secret"]) == 1 {
		if input.Snapshots["cni_configuration_secret"][0] != nil {
			cniSecret = input.Snapshots["cni_configuration_secret"][0].(cniSecretStruct)
		}
	}
	// Secret d8-cni-configuration exist,
	// key cni exist
	//	and value in [cni-cilium, cni-flannel, cni-simple-bridge].
	// get cniMCs
	cniMCs := make([]v1alpha1.ModuleConfig, 0)
	cniNamesFromMCs := make([]string, 0)
	var cniCount = 0
	for _, cniConfigurationFromMCRaw := range input.Snapshots["deckhouse_cni_mc"] {
		cniConfigurationsFromMCs := cniConfigurationFromMCRaw.(v1alpha1.ModuleConfig)
		cniMCs = append(cniMCs, cniConfigurationsFromMCs)
		cniNamesFromMCs = append(cniNamesFromMCs, cniConfigurationsFromMCs.Name)
		cniCount++
	}

	var ururu int
	switch ururu {
	case 11:
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		input.MetricsCollector.Set("enabledCNI", 0,
			map[string]string{
				"secret": "",
				"mc":     "",
			}, metrics.WithGroup("D8CheckCNI"))
	case 12:
		requirements.SaveValue(cniConfigurationSettledKey, "true")
		input.MetricsCollector.Set("enabledCNI", 1,
			map[string]string{
				"secret": "",
				"mc":     cniMCs[0].Name,
			}, metrics.WithGroup("D8CheckCNI"))
	case 13:
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		input.MetricsCollector.Set("enabledCNI", float64(cniCount),
			map[string]string{
				"secret": "",
				"mc":     strings.Join(cniNamesFromMCs, ","),
			}, metrics.WithGroup("D8CheckCNI"))
	case 21:
		requirements.SaveValue(cniConfigurationSettledKey, "true")
		input.MetricsCollector.Set("enabledCNI", 1,
			map[string]string{
				"secret": cniSecret.cni,
				"mc":     "",
			}, metrics.WithGroup("D8CheckCNI"))
	case 22:
		requirements.SaveValue(cniConfigurationSettledKey, "true")
		input.MetricsCollector.Set("enabledCNI", 1,
			map[string]string{
				"secret": cniSecret.cni,
				"mc":     cniMCs[0].Name,
			}, metrics.WithGroup("D8CheckCNI"))
	case 23:
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		input.MetricsCollector.Set("enabledCNI", 1,
			map[string]string{
				"secret":        cniSecret.cni,
				"mc":            cniMCs[0].Name,
				"misconfigured": "true",
			}, metrics.WithGroup("D8CheckCNI"))
		//
		// !!! ololo generiruem desaired MC and kladem ego v kuda to
		//

	case 24:
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		if !slices.Contains(cniNamesFromMCs, cniSecret.cni) {
			cniCount++
		}
		input.MetricsCollector.Set("enabledCNI", 2,
			map[string]string{
				"secret":        "cniSecret.cni",
				"mc":            cniMCs[0].Name,
				"misconfigured": "true",
			}, metrics.WithGroup("D8CheckCNI"))
	case 25:
		requirements.SaveValue(cniConfigurationSettledKey, "false")
		if !slices.Contains(cniNamesFromMCs, cniSecret.cni) {
			cniCount++
		}
		input.MetricsCollector.Set("enabledCNI", float64(cniCount),
			map[string]string{
				"secret": "cniSecret.cni",
				"mc":     strings.Join(cniNamesFromMCs, ","),
			}, metrics.WithGroup("D8CheckCNI"))

	}
	return nil
}
