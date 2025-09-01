/*
Copyright 2021 Flant JSC

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

type FlannelConfigStruct struct {
	PodNetworkMode string `json:"podNetworkMode"`
}

type resultStruct struct {
	DesiredCniConfigSourcePriorityFlagExists bool
	DesiredCniConfigSourcePriority           string
	CniConfigFromSecret                      FlannelConfigStruct
}

func applyCNIConfigurationSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &v1.Secret{}
	err := sdk.FromUnstructured(obj, secret)
	if err != nil {
		return nil, fmt.Errorf("cannot convert incoming object to Secret: %v", err)
	}

	if string(secret.Data["cni"]) != "flannel" {
		return nil, nil
	}

	var flannelConfig FlannelConfigStruct
	flannelConfigJSON, ok := secret.Data["flannel"]
	if !ok {
		return nil, nil
	}

	err = json.Unmarshal(flannelConfigJSON, &flannelConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal flannel config json: %v", err)
	}

	cniConfigSourcePriorityFlagExists := false
	cniConfigSourcePriority := ""
	cniConfigSourcePriority, cniConfigSourcePriorityFlagExists = secret.Annotations[cniConfigSourcePriorityAnnotation]

	return resultStruct{
		DesiredCniConfigSourcePriorityFlagExists: cniConfigSourcePriorityFlagExists,
		DesiredCniConfigSourcePriority:           cniConfigSourcePriority,
		CniConfigFromSecret:                      flannelConfig,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-flannel",
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
}, setPodNetworkMode)

func setPodNetworkMode(_ context.Context, input *go_hook.HookInput) error {
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
		flannelConfig := cniConfigurationSecrets[0].CniConfigFromSecret
		if flannelConfig.PodNetworkMode != "" {
			input.Values.Set("cniFlannel.internal.podNetworkMode", flannelConfig.PodNetworkMode)
		}
		return nil
	case "ModuleConfig":
		value, ok := input.ConfigValues.GetOk("cniFlannel.podNetworkMode")
		if ok {
			switch value.String() {
			case "HostGW":
				input.Values.Set("cniFlannel.internal.podNetworkMode", "host-gw")
			case "VXLAN":
				input.Values.Set("cniFlannel.internal.podNetworkMode", "vxlan")
			}
		}
	}

	// default_podNetworkMode = HostGW
	return nil
}
