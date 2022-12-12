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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

/*
This hook enables cni module enabled either explicitly in configuration or
during installation in Secret/d8-cni-configuration.

Developer notes:
- It uses "dynamic enable" feature of addon-operator to enable module in runtime.
- It executes on Synchronization to return values patch before ConvergeModules task.
- It is the only hook that subscribes to configuration ConfigMap because
  there is no way to get enabled modules list in global hook.
*/

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
		{
			Name:       "deckhouse_cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{os.Getenv("ADDON_OPERATOR_CONFIG_MAP")},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   applyD8CMFilter,
		},
	},
}, enableCni)

func applyD8CMFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return "", err
	}

	cniMap := make(map[string]bool)

	for k, v := range cm.Data {
		// looking for keys like 'cniCiliumEnabled' or 'cniFlannelEnabled'
		if strings.HasPrefix(k, "cni") && strings.HasSuffix(k, "Enabled") {
			boolValue, err := strconv.ParseBool(v)
			if err != nil {
				return nil, fmt.Errorf("parse cni enable flag failed: %s", err)
			}
			cniMap[k] = boolValue
		}
	}

	return cniMap, nil
}

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
	deckhouseCMSnap := input.Snapshots["deckhouse_cm"]

	if len(cniNameSnap) == 0 {
		input.LogEntry.Warnln("Cni name not found")
		return nil
	}

	if len(deckhouseCMSnap) == 0 {
		input.LogEntry.Warnln("Deckhouse CM not found")
		return nil
	}

	cmEnabledCNIs := make([]string, 0)
	for cni, enabled := range deckhouseCMSnap[0].(map[string]bool) {
		if enabled {
			cmEnabledCNIs = append(cmEnabledCNIs, cni)
		}
	}

	if len(cmEnabledCNIs) > 1 {
		return fmt.Errorf("more then one CNI enabled: %v", cmEnabledCNIs)
	} else if len(cmEnabledCNIs) == 1 {
		input.LogEntry.Infof("enabled CNI from Deckhouse CM: %s", strings.TrimSuffix(cmEnabledCNIs[0], "Enabled"))
		return nil
	}

	// nor any CNI enabled directly via CM, found default CNI from secret
	cniToEnable := cniNameSnap[0].(string)
	if _, ok := cniNameToModule[cniToEnable]; !ok {
		input.LogEntry.Warnf("Incorrect cni name: '%v'. Skip", cniToEnable)
		return nil
	}

	input.LogEntry.Infof("enabled CNI by secret: %s", cniToEnable)
	input.Values.Set(cniNameToModule[cniToEnable], true)
	return nil
}
