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

package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

const configOverridesTemplate = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.28"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: aaaa
{{- if .bundle }}
  bundle: {{ .bundle }}
{{- end }}

{{- if .releaseChannel }}
  releaseChannel: {{ .releaseChannel }}
{{- end }}

{{- if .logLevel }}
  logLevel: {{ .logLevel }}
{{- end }}

{{- if .configOverrides}}
{{- .configOverrides | nindent 2 }}
{{- end }}
---
apiVersion: deckhouse.io/v1alpha1
# type of the configuration section
kind: StaticClusterConfiguration
# address space for the cluster's internal network
internalNetworkCIDRs:
- 192.168.199.0/24
---
{{- if .moduleConfigs}}
{{- .moduleConfigs }}
{{- end }}
`

func assertModuleConfig(t *testing.T, mc *ModuleConfig, enabled bool, version int, setting map[string]interface{}) {
	require.NotNil(t, mc.Spec.Enabled)
	require.Equal(t, *mc.Spec.Enabled, enabled)
	require.Equal(t, mc.Spec.Version, version)
	require.Equal(t, mc.Spec.Settings, SettingsValues(setting))
}

func TestCheckOrSetupArbitaryCNIModuleConfig(t *testing.T) {
	type result struct {
		cniName  string
		enabled  bool
		version  int
		settings SettingsValues
	}

	type testProvider struct {
		cloudProvider  string
		podNetworkMode string
		result         result
	}

	testCases := []testProvider{
		{
			cloudProvider:  "AWS",
			podNetworkMode: "",
			result: result{
				cniName:  "cni-simple-bridge",
				enabled:  true,
				version:  1,
				settings: nil,
			},
		},
		{
			cloudProvider:  "GCP",
			podNetworkMode: "",
			result: result{
				cniName:  "cni-simple-bridge",
				enabled:  true,
				version:  1,
				settings: nil,
			},
		},
		{
			cloudProvider:  "Azure",
			podNetworkMode: "",
			result: result{
				cniName:  "cni-simple-bridge",
				enabled:  true,
				version:  1,
				settings: nil,
			},
		},
		{
			cloudProvider:  "Yandex",
			podNetworkMode: "",
			result: result{
				cniName:  "cni-simple-bridge",
				enabled:  true,
				version:  1,
				settings: nil,
			},
		},
		{
			cloudProvider:  "Openstack",
			podNetworkMode: "VXLAN",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":     "VXLAN",
					"masqueradeMode": "BPF",
				},
			},
		},
		{
			cloudProvider:  "Openstack",
			podNetworkMode: "Direct",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":       "Disabled",
					"masqueradeMode":   "Netfilter",
					"createNodeRoutes": true,
				},
			},
		},
		{
			cloudProvider:  "VCD",
			podNetworkMode: "",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":       "Disabled",
					"masqueradeMode":   "Netfilter",
					"createNodeRoutes": true,
				},
			},
		},
		{
			cloudProvider:  "ZVirt",
			podNetworkMode: "",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":       "Disabled",
					"masqueradeMode":   "Netfilter",
					"createNodeRoutes": true,
				},
			},
		},
		{
			cloudProvider:  "Vsphere",
			podNetworkMode: "",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":       "Disabled",
					"masqueradeMode":   "Netfilter",
					"createNodeRoutes": true,
				},
			},
		},
		// static cluster
		{
			cloudProvider:  "",
			podNetworkMode: "",
			result: result{
				cniName: "cni-cilium",
				enabled: true,
				version: 1,
				settings: map[string]interface{}{
					"tunnelMode":       "Disabled",
					"masqueradeMode":   "Netfilter",
					"createNodeRoutes": true,
				},
			},
		},
	}

	for _, p := range testCases {
		t.Run("Check generation of the cni module config for "+p.cloudProvider, func(t *testing.T) {
			cfg := &DeckhouseInstaller{
				ProviderClusterConfig: []byte(fmt.Sprintf("{\"kind\": \"%sClusterConfiguration\"}", p.cloudProvider)),
			}
			if p.podNetworkMode != "" {
				cfg.CloudDiscovery = []byte(fmt.Sprintf("{\"podNetworkMode\": \"%s\"}", p.podNetworkMode))
			}

			err := CheckOrSetupArbitaryCNIModuleConfig(cfg)
			require.NoError(t, err)
			require.Equal(t, cfg.ModuleConfigs[0].GetName(), p.result.cniName)
			assertModuleConfig(t, cfg.ModuleConfigs[0], p.result.enabled, p.result.version, p.result.settings)
		})
	}
}
