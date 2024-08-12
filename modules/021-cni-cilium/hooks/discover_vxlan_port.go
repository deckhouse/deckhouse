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
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

type Installation int

const (
	Existing Installation = iota
	New
)

type Virtualization int

const (
	On Virtualization = iota
	Off
)

var transitions = map[Installation]map[Virtualization][]TransitionRule{
	Existing: { // existing installation (ConfigMap exists)
		On: { // Virtualization is on
			// cm has configured 8469 port, will leave it as is
			TransitionRule{source: 8469, target: 8469},

			// dreamy case — virtualization was enabled with upgrading d8 simultaneously
			TransitionRule{source: 0, target: 8469},

			// dreamy case — virtualization was enabled with upgrading d8 simultaneously and
			// someone configured the port for setup without virtualization 8472 manually, will set the right one
			TransitionRule{source: 8472, target: 4298},

			// virtualization module was enabled on regular setup with the right port, will set the 4298
			TransitionRule{source: 4299, target: 4298},

			// regular setup with enabled virtualization module and right port, will leave it as is
			TransitionRule{source: 4298, target: 4298},

			// if the "source" port is non-standard and didn't mention here, will leave it as is and fire the alert
		},
		Off: { // Virtualization is off
			// our previous standard setup, will set the old default port explicitly
			TransitionRule{source: 0, target: 8472},

			// our previous standard setup with explicitly configured 8472 port, will leave the 8472
			TransitionRule{source: 8472, target: 8472},

			// regular setup with standard 4299 port, will leave it as is
			TransitionRule{source: 4299, target: 4299},
		},
	},
	New: { // new installation (ConfigMap does not exist)
		On: { // Virtualization is on
			TransitionRule{source: 0, target: 4298},
		},
		Off: { // Virtualization is off
			TransitionRule{source: 0, target: 4299},
		},
	},
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/cni-cilium/discover_vxlan_port",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cilium-configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"cilium-config"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cni-cilium"},
				},
			},
			FilterFunc: filterConfigMap,
		},
	},
}, discoverVXLANPort)

func filterConfigMap(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm v1.ConfigMap
	cmInfo := ConfigMapInfo{}

	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, err
	}

	if port, exist := cm.Data["tunnel-port"]; exist {
		cmInfo.Port, _ = strconv.Atoi(port)
	}
	return cmInfo, nil
}

func discoverVXLANPort(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_cni_cilium_config")
	var installationStatus = New

	if len(input.Snapshots["cilium-configmap"]) > 0 {
		installationStatus = Existing
	}

	var virtualizationModuleStatus = Off
	enabledModules := set.NewFromValues(input.Values, "global.enabledModules")
	if enabledModules.Has("virtualization") {
		virtualizationModuleStatus = On
	}

	var targetPort, sourcePort int
	if len(input.Snapshots["cilium-configmap"]) > 0 {
		cm := input.Snapshots["cilium-configmap"][0].(ConfigMapInfo)
		sourcePort = cm.Port
	}

	var transitionFound bool
	for _, rule := range transitions[installationStatus][virtualizationModuleStatus] {
		if rule.source == sourcePort {
			targetPort = rule.target
			transitionFound = true
			break
		}
	}

	if !transitionFound {
		targetPort = sourcePort
		input.MetricsCollector.Set("d8_cni_cilium_non_standard_vxlan_port", 1, map[string]string{"port": fmt.Sprintf("%d", targetPort)}, metrics.WithGroup("d8_cni_cilium_config"))
	}

	input.Values.Set("cniCilium.internal.tunnelPortVXLAN", targetPort)

	return nil
}

type ConfigMapInfo struct {
	Port int
}

type DaemonSetInfo struct{}

type TransitionRule struct {
	source int
	target int
}
