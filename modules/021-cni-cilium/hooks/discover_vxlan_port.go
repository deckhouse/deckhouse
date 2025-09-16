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
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	metricGroupVXLANPort           = "d8_cni_cilium_config_vxlan_port"
	metricNameNonStandardVXLANPort = "d8_cni_cilium_non_standard_vxlan_port"
	defaultVXLANPort               = 4298
)

type installationStatus int

type transitionRule struct {
	source int
	target int
}

const (
	existingInstallation installationStatus = iota
	newInstallation
)

type virtualizationStatus int

const (
	virtualizationEnabled virtualizationStatus = iota
	virtualizationDisabled
)

func getTransitionRules(instStatus installationStatus, virtStatus virtualizationStatus, virtNestingLevel int) []transitionRule {
	switch instStatus {
	case existingInstallation: // (ConfigMap exists)
		switch {
		case virtNestingLevel > 0: // (Nested installation)
			return []transitionRule{
				// regular setup with certain nesting level
				{source: defaultVXLANPort - virtNestingLevel, target: defaultVXLANPort - virtNestingLevel},

				// empty configmap for some reason - reset the port considering the nesting level
				{source: 0, target: defaultVXLANPort - virtNestingLevel},
			}

		default:
			switch virtStatus {
			case virtualizationEnabled:
				return []transitionRule{
					// cm has configured 8469 port, will leave it as is
					{source: 8469, target: 8469},

					// dreamy case — virtualization was enabled with upgrading d8 simultaneously
					{source: 0, target: 8469},

					// dreamy case — virtualization was enabled with upgrading d8 simultaneously and
					// someone configured the port for setup without virtualization 8472 manually, will set the right one
					{source: 8472, target: defaultVXLANPort},

					// virtualization module was enabled on regular setup with the right port, will set the defaultVXLANPort port
					{source: 4299, target: defaultVXLANPort},

					// regular setup with enabled virtualization module and right port, will leave it as is
					{source: defaultVXLANPort, target: defaultVXLANPort},

					// if the "source" port is non-standard and didn't mention here, will leave it as is and fire the alert
				}

			case virtualizationDisabled:
				return []transitionRule{
					// our previous standard setup, will set the old default port explicitly
					{source: 0, target: 8472},

					// our previous standard setup with explicitly configured 8472 port, will leave the 8472
					{source: 8472, target: 8472},

					// virtualizaiton module was disabled on regular setup with standard defaultVXLANPort port, will set the port in accordance with the nesting level
					{source: defaultVXLANPort, target: defaultVXLANPort - virtNestingLevel},

					// regular setup with standard 4299 port, will leave it as is
					{source: 4299, target: 4299},
				}
			}
		}

	case newInstallation: // (ConfigMap does not exist)
		return []transitionRule{
			{source: 0, target: defaultVXLANPort - virtNestingLevel},
		}
	}

	return nil
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
	cm := new(v1.ConfigMap)

	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, err
	}

	if port, ok := cm.Data["tunnel-port"]; ok {
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return 0, nil
		}

		return portInt, nil
	}

	return 0, nil
}

func discoverVXLANPort(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(metricGroupVXLANPort)

	var (
		instStatus       = newInstallation
		virtStatus       = virtualizationDisabled
		virtNestingLevel int
		targetPort       int
		sourcePort       int
		transitionFound  bool
	)

	virtNestingLevelRaw, ok := input.Values.GetOk("global.discovery.dvpNestingLevel")
	if ok {
		virtNestingLevel = int(virtNestingLevelRaw.Int())
	} else {
		input.Logger.Warn("Virtualization nesting level is not set globally - assuming level 0")
	}

	ports, err := sdkobjectpatch.UnmarshalToStruct[int](input.Snapshots, "cilium-configmap")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'cilium-configmap' snapshot: %w", err)
	}

	if len(ports) > 0 {
		instStatus = existingInstallation
		if port := ports[0]; port > 0 {
			sourcePort = port
		}
	}

	if set.NewFromValues(input.Values, "global.enabledModules").Has("virtualization") {
		virtStatus = virtualizationEnabled
	}

	for _, rule := range getTransitionRules(instStatus, virtStatus, virtNestingLevel) {
		if rule.source == sourcePort {
			targetPort = rule.target
			transitionFound = true
			break
		}
	}

	if !transitionFound {
		targetPort = sourcePort
		input.MetricsCollector.Set(metricNameNonStandardVXLANPort, 1, map[string]string{
			"current_port":     fmt.Sprintf("%d", targetPort),
			"recommended_port": fmt.Sprintf("%d", defaultVXLANPort-virtNestingLevel),
		}, metrics.WithGroup(metricGroupVXLANPort))
	}

	input.Values.Set("cniCilium.internal.tunnelPortVXLAN", targetPort)

	return nil
}
