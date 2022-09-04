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
	"encoding/base64"
	"fmt"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type cniConfigStruct struct {
	cni    string
	cilium []byte
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
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
			FilterFunc: applyCniConfigFilter,
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
	ret.cni = string(s.Data["cni"])
	ret.cilium = s.Data["cilium"]
	return ret, nil
}

func migrateCniConfig(input *go_hook.HookInput) error {
	cniConfigSnap := input.Snapshots["cni_config"]

	if len(cniConfigSnap) == 0 {
		input.LogEntry.Warnln("kube-system/d8-cni-configuration secret data not found, skip migration")
		return nil
	}

	cniConfig := cniConfigSnap[0].(cniConfigStruct)
	if cniConfig.cni != "cilium" {
		input.LogEntry.Warnf("ckube-system/d8-cni-configuration secret cni type = %s, skip migration", cniConfig.cni)
		return nil
	}

	if cniConfig.cilium != nil {
		input.LogEntry.Warnln("ckube-system/d8-cni-configuration secret cilium config is present, skip migration")
		return nil
	}

	if input.ConfigValues.Get("cniCilium.tunnelMode").String() == "VXLAN" {
		patchCniConfigSecret(input, "VXLAN")
		return nil
	}

	value, ok := input.ConfigValues.GetOk("cniCilium.createNodeRoutes")
	if ok {
		if value.Bool() {
			patchCniConfigSecret(input, "DirectWithNodeRoutes")
			return nil
		}
		patchCniConfigSecret(input, "Direct")
		return nil
	}

	providerRaw, ok := input.Values.GetOk("global.clusterConfiguration.cloud.provider")
	if ok {
		switch strings.ToLower(providerRaw.String()) {
		case "openstack", "vsphere":
			patchCniConfigSecret(input, "DirectWithNodeRoutes")
			return nil
		}
	}

	patchCniConfigSecret(input, "Direct")
	return nil
}

func patchCniConfigSecret(input *go_hook.HookInput, mode string) {

	modeJSON := fmt.Sprintf("{\"mode\": \"%s\"}", mode)
	var (
		patch = map[string]interface{}{
			"data": map[string]string{
				"cilium": base64.StdEncoding.EncodeToString([]byte(modeJSON)),
			},
		}
	)

	input.PatchCollector.MergePatch(patch, "v1", "Secret", "kube-system", "d8-cni-configuration")
}
