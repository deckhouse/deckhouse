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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

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

func applyCNIConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}
	cniSecret := cniSecretStruct{}
	cniBytes, ok := secret.Data["cni"]
	if !ok {
		return nil, fmt.Errorf("d8-cni-configuration secret does not contain \"cni\" field")
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
		return nil, fmt.Errorf("unknown cni name: %s", cniSecret.cni)
	}
}

func applyCNIMCFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	v, _, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "enabled")
	if err != nil {
		return nil, err
	}

	if !v {
		return nil, nil
	}
	return obj.GetName(), nil
}
func checkCni(input *go_hook.HookInput) error {
	cniConfigurationsFromSecrets, ok := input.Snapshots["cni_configuration_secret"]
	cniConfigurationsFromMCs, ok := input.Snapshots["deckhouse_cni_mc"]
	return nil
}
