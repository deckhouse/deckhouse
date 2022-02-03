// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	cniNameToModule = map[string]string{
		"flannel":       "cniFlannelEnabled",
		"simple-bridge": "cniSimpleBridgeEnabled",
		"cilium":        "cniCiliumEnabled",
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cni_name",
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
}, enableCni)

func applyCniConfigFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.Secret
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	cni, ok := cm.Data["cni"]
	if ok {
		return string(cni), nil
	}

	return nil, nil
}

func enableCni(input *go_hook.HookInput) error {
	cniNameSnap := input.Snapshots["cni_name"]

	if len(cniNameSnap) == 0 {
		input.LogEntry.Warnln("Cni name not found")
		return nil
	}

	cniToEnable := cniNameSnap[0].(string)
	if _, ok := cniNameToModule[cniToEnable]; !ok {
		input.LogEntry.Warnf("Incorrect cni name: '%v'. Skip", cniToEnable)
		return nil
	}

	for cniName, module := range cniNameToModule {
		_, ok := input.ConfigValues.GetOk(module)
		if ok {
			continue
		}

		if cniToEnable == cniName {
			input.Values.Set(module, true)
		} else {
			input.Values.Remove(module)
		}
	}

	return nil
}
