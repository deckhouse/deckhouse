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
	"log/slog"
	"time"

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
	cniConfigurationSettledKey        = "cniConfigurationSettled"
	checkCNIConfigMetricName          = "cniMisconfigured"
	checkCNIConfigMetricGroup         = "d8_check_cni_conf"
	desiredCNIModuleConfigName        = "desired-cni-moduleconfig"
	cni                               = "cilium"
	cniName                           = "cni-" + cni
	cniConfigurationIsNotSettled      = true
	cniConfigurationIsSettled         = false
	cniConfigSourcePriorityAnnotation = "network.deckhouse.io/cni-configuration-source-priority"
)

type flannelConfigStruct struct {
	PodNetworkMode string `json:"podNetworkMode"`
}

type ciliumConfigStruct struct {
	Mode           string `json:"mode,omitempty"`
	MasqueradeMode string `json:"masqueradeMode,omitempty"`
}

type cniSecretStruct struct {
	CreationTimestamp                 time.Time
	CniConfigSourcePriorityFlagExists bool
	CNI                               string
	Flannel                           flannelConfigStruct
	Cilium                            ciliumConfigStruct
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
	// an error occurred while JSON parse
	// or d8-cni-configuration secret does not contain key "cni"
	// or value of key "cni" not in [cni-cilium, cni-flannel, cni-simple-bridge]
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}
	cniSecret := cniSecretStruct{}

	// get creation timestamp from secret
	cniSecret.CreationTimestamp = secret.CreationTimestamp.Time

	// Check if the secret has the annotation "network.deckhouse.io/cni-configuration-source-priority"
	_, exists := secret.Annotations[cniConfigSourcePriorityAnnotation]
	cniSecret.CniConfigSourcePriorityFlagExists = exists

	cniBytes, ok := secret.Data["cni"]
	if !ok {
		// d8-cni-configuration secret does not contain the "cni" field
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

	return mc, nil
}

func checkCni(_ context.Context, input *go_hook.HookInput) error {
	// Clear a metrics and reqKey
	input.MetricsCollector.Expire(checkCNIConfigMetricGroup)
	requirements.RemoveValue(cniConfigurationSettledKey)

	// Get existing secret.
	cniSecrets, err := sdkobjectpatch.UnmarshalToStruct[cniSecretStruct](input.Snapshots, "cni_configuration_secret")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cni_configuration_secret snapshot: %w", err)
	}

	// If secret does not exist, then we are already using a new logic de facto: the parameters in the MC have priority.
	// So there is nothing to do.
	if len(cniSecrets) == 0 {
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// If the secret has the annotation "network.deckhouse.io/cni-configuration-source-priority", so there is nothing to do.
	if cniSecrets[0].CniConfigSourcePriorityFlagExists {
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// If secret does not contain config for current cni, then we are already using a new logic de facto: the parameters in the MC have priority.
	// - add an annotation to the secret
	cniSecret := cniSecrets[0]
	if cniSecret.CNI != cni {
		annotateSecret(input)
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// Get existing MC.
	cniModuleConfigs, err := sdkobjectpatch.UnmarshalToStruct[v1alpha1.ModuleConfig](input.Snapshots, "deckhouse_cni_mc")
	if err != nil {
		return fmt.Errorf("failed to unmarshal deckhouse_cni_mc snapshot: %w", err)
	}

	// Prepare a template for the desiredCNIModuleConfig, which is empty and explicitly enabled.
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
			Settings: &v1alpha1.SettingsValues{},
		},
	}
	// If the MC exists, use its Settings to generate the desired MC.
	if len(cniModuleConfigs) != 0 {
		cniModuleConfig := cniModuleConfigs[0]
		desiredCNIModuleConfig.Spec.Settings = cniModuleConfig.DeepCopy().Spec.Settings
	}

	var settings map[string]any
	err = json.Unmarshal(desiredCNIModuleConfig.Spec.Settings.Raw, &settings)
	if err != nil {
		return fmt.Errorf("cannot unmarshal settings of ModuleConfig %q: %w", desiredCNIModuleConfig.Name, err)
	}

	// Generate the desired CNIModuleConfig based on existing secret and MC and compare them at the same time.
	// Skip if in the secret key "cilium" does not exist or empty.
	secretMatchesMC := true
	if cniSecret.Cilium != (ciliumConfigStruct{}) {
		switch cniSecret.Cilium.Mode {
		case "VXLAN":
			value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode")
			if !ok || value.String() != "VXLAN" {
				settings["tunnelMode"] = "VXLAN"
				secretMatchesMC = false
			}
		case "Direct":
			value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode")
			if !ok || value.String() != "Disabled" {
				settings["tunnelMode"] = "Disabled"
				secretMatchesMC = false
			}
		case "DirectWithNodeRoutes":
			value, ok := input.ConfigValues.GetOk("cniCilium.tunnelMode")
			if !ok || value.String() != "Disabled" {
				settings["tunnelMode"] = "Disabled"
				secretMatchesMC = false
			}
			value, ok = input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
			if !ok || !value.Bool() {
				settings["createNodeRoutes"] = true
				secretMatchesMC = false
			}
		default:
			input.Logger.Warn("An unknown cilium mode was specified in the d8-cni-configuration secret, so the default cni mode will be used instead.", slog.String("specified mode", cniSecret.Cilium.Mode))
		}

		switch cniSecret.Cilium.MasqueradeMode {
		case "Netfilter", "BPF":
			value, ok := input.ConfigValues.GetOk("cniCilium.masqueradeMode")
			if !ok || value.String() != cniSecret.Cilium.MasqueradeMode {
				settings["masqueradeMode"] = cniSecret.Cilium.MasqueradeMode
				secretMatchesMC = false
			}
		case "":
			value, ok := input.ConfigValues.GetOk("cniCilium.masqueradeMode")
			if !ok || value.String() != "BPF" {
				settings["masqueradeMode"] = "BPF"
				secretMatchesMC = false
			}
		default:
			input.Logger.Warn("An unknown cilium masqueradeMode was specified in the d8-cni-configuration secret, so the default cni masqueradeMode will be used instead.", slog.String("specified masqueradeMode", cniSecret.Cilium.Mode))
		}
	}

	// If MC does not exist, then we should
	// - add an annotation to the secret (to activate new_logic)
	// - create the desired MC, which was generated based on the secret.
	if len(cniModuleConfigs) == 0 {
		annotateSecret(input)
		createDesiredModuleConfig(input, desiredCNIModuleConfig)
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// If MC exist, but is explicitly disabled, it means that CNI is in the process of disabling, there is nothing to do.
	if cniModuleConfigs[0].Spec.Enabled != nil && !*cniModuleConfigs[0].Spec.Enabled {
		return nil
	}

	if cniModuleConfigs[0].Spec.Enabled == nil {
		secretMatchesMC = false
	}

	// If the secret matches MC, then we should
	// - add an annotation to the secret (to activate new_logic)
	if secretMatchesMC {
		annotateSecret(input)
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// Let's check if the cluster has already been bootstrapped.
	clusterIsBootstrapped := input.Values.Get("global.clusterIsBootstrapped").Bool()

	// If the cluster is not yet bootstrapped, then we should
	// - add an annotation to the secret (to activate new_logic)
	if !clusterIsBootstrapped {
		annotateSecret(input)
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// Let's check what was created earlier: MC(+10m) or Secret.
	if cniSecret.CreationTimestamp.After(cniModuleConfigs[0].CreationTimestamp.Time.Add(10 * time.Minute)) {
		annotateSecret(input)
		setMetricAndRequirementsValue(input, cniConfigurationIsSettled)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-system", desiredCNIModuleConfigName)
		return nil
	}

	// If the cluster is already bootstrapped and the secret was created earlier than MC, then we should
	// - generate desired MC based on secret
	// - create cm based on desired MC
	// - fire alert
	err = createConfigMapWithDesiredModuleConfig(input, desiredCNIModuleConfig)
	if err != nil {
		return fmt.Errorf("failed to create config map with desired module config: %w", err)
	}
	setMetricAndRequirementsValue(input, cniConfigurationIsNotSettled)
	return nil
}

func setMetricAndRequirementsValue(input *go_hook.HookInput, isCniMisconfigured bool) {
	switch isCniMisconfigured {
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

func annotateSecret(input *go_hook.HookInput) {
	secretPatch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"network.deckhouse.io/cni-configuration-source-priority": "ModuleConfig",
			},
		},
	}
	input.PatchCollector.PatchWithMerge(secretPatch, "v1", "Secret", "kube-system", "d8-cni-configuration")
}

func createDesiredModuleConfig(input *go_hook.HookInput, desiredCNIModuleConfig *v1alpha1.ModuleConfig) {
	input.PatchCollector.CreateOrUpdate(desiredCNIModuleConfig)
}

func createConfigMapWithDesiredModuleConfig(input *go_hook.HookInput, desiredCNIModuleConfig *v1alpha1.ModuleConfig) error {
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
	return nil
}
