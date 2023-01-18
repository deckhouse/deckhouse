/*
Copyright 2023 Flant JSC

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

/*
Some features as backup shipping and luks encryption requires master passphrase set
This hook reads secret d8-system/linstor-passphrase and specifies it for LINSTOR.
*/

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

func applyCiliumTunnelFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, fmt.Errorf("cannot convert cilium configmap: %v", err)
	}

	if cm.Data["tunnel"] == "" || cm.Data["tunnel"] == "disabled" {
		return false, nil
	}

	return true, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cilium_tunnel",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cni-cilium"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cilium-config"},
			},
			FilterFunc:          applyCiliumTunnelFilter,
			ExecuteHookOnEvents: pointer.BoolPtr(false),
		},
	},
}, applyRouteLocal)

func applyRouteLocal(input *go_hook.HookInput) error {
	snaps := input.Snapshots["cilium_tunnel"]
	var ciliumTunnel bool
	for _, snap := range snaps {
		ciliumTunnel = snap.(bool)
	}
	input.Values.Set("virtualization.internal.routeLocal", ciliumTunnel)
	return nil
}
