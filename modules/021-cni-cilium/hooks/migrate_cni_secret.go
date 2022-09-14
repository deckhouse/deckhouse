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

/*
This hook migrates kube-system/d8-cni-configuration secret for cilium configs.
Date of migration: 06.09.2022
TODO: remove this migration on release 1.37
*/
package hooks

import (
	"encoding/base64"
	"encoding/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

type cniConfigStruct struct {
	Cni    string
	Cilium []byte
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cni_config",
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
			FilterFunc:                   applyCniConfigFilter,
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
		},
	},
}, migrateCniConfig)

func applyCniConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var s v1core.Secret
	err := sdk.FromUnstructured(obj, &s)
	if err != nil {
		return nil, err
	}
	ret := cniConfigStruct{}
	ret.Cni = string(s.Data["cni"])
	ret.Cilium = s.Data["cilium"]
	return ret, nil
}

func migrateCniConfig(input *go_hook.HookInput) error {
	cniConfigSnap := input.Snapshots["cni_config"]

	if len(cniConfigSnap) == 0 {
		input.LogEntry.Warnln("kube-system/d8-cni-configuration secret data not found, skip migration")
		return nil
	}

	cniConfig := cniConfigSnap[0].(cniConfigStruct)
	if cniConfig.Cni != "cilium" {
		input.LogEntry.Warnf("kube-system/d8-cni-configuration secret cni type = %s, skip migration", cniConfig.Cni)
		return nil
	}

	if cniConfig.Cilium != nil {
		input.LogEntry.Warnln("kube-system/d8-cni-configuration secret cilium config is present, skip migration")
		return nil
	}

	var masqueradeMode string
	if input.ConfigValues.Get("cniCilium.tunnelMode").String() == "VXLAN" {
		return patchCniConfigSecret(input, "VXLAN", masqueradeMode)
	}

	// if cloud provider == Openstack we should set masqueradeMode to Netfilter
	if value, ok := input.Values.GetOk("global.clusterConfiguration.cloud.provider"); ok && value.String() == "OpenStack" {
		masqueradeMode = "Netfilter"
	}

	value, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok {
		if value.Bool() {
			return patchCniConfigSecret(input, "DirectWithNodeRoutes", masqueradeMode)
		}
		return patchCniConfigSecret(input, "Direct", masqueradeMode)
	}

	return patchCniConfigSecret(input, "DirectWithNodeRoutes", masqueradeMode)
}

func patchCniConfigSecret(input *go_hook.HookInput, mode string, masqueradeMode string) error {
	jsonByte, err := generateJSONCiliumConf(mode, masqueradeMode)
	if err != nil {
		return err
	}
	var (
		patch = map[string]interface{}{
			"data": map[string]string{
				"cilium": base64.StdEncoding.EncodeToString([]byte(jsonByte)),
			},
		}
	)
	input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", "d8-cni-configuration")
	return nil
}

func generateJSONCiliumConf(mode string, masqueradeMode string) ([]byte, error) {
	var confMAP CiliumConfigStruct
	if mode != "" {
		confMAP.Mode = mode
	}
	if masqueradeMode != "" {
		confMAP.MasqueradeMode = masqueradeMode
	}

	return json.Marshal(confMAP)
}
