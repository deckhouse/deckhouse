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

func generateMetaConfigForConfigOverridesTest(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data, false)
}

func assertModuleConfig(t *testing.T, mc *ModuleConfig, enabled bool, version int, setting map[string]interface{}) {
	require.NotNil(t, mc.Spec.Enabled)
	require.Equal(t, *mc.Spec.Enabled, enabled)
	require.Equal(t, mc.Spec.Version, version)
	require.Equal(t, mc.Spec.Settings, SettingsValues(setting))
}

func TestModuleConfigOverridesToModuleConfig(t *testing.T) {
	t.Run("All valid", func(t *testing.T) {
		metaConfig := generateMetaConfigForConfigOverridesTest(t, map[string]interface{}{
			"configOverrides": `
configOverrides:
  istioEnabled: false
  global:
    modules:
      publicDomainTemplate: "%s.example.com"
  cniCiliumEnabled: true
  cniCilium:
    tunnelMode: VXLAN
  common:
    testString: aaaaa
`,
		})

		mcs, err := ConvertInitConfigurationToModuleConfigs(metaConfig, NewSchemaStore(), "Minimal", "Debug")
		require.NoError(t, err)

		foundDeckhouseCm := false
		for _, mc := range mcs {
			switch mc.GetName() {
			case "istioEnabled":
				assertModuleConfig(t, mc, false, 1, nil)
			case "global":
				assertModuleConfig(t, mc, true, 1, map[string]interface{}{
					"modules": map[string]interface{}{
						"publicDomainTemplate": "%s.example.com",
					},
				})
			case "cniCilium":
				assertModuleConfig(t, mc, true, 1, map[string]interface{}{
					"tunnelMode": "VXLAN",
				})

			case "common":
				assertModuleConfig(t, mc, true, 1, map[string]interface{}{
					"testString": "aaaaa",
				})

			case "deckhouse":
				foundDeckhouseCm = true
				assertModuleConfig(t, mc, true, 1, map[string]interface{}{
					"bundle":   "Minimal",
					"logLevel": "Debug",
				})
			}
		}

		require.True(t, foundDeckhouseCm)
	})

	t.Run("Invalid overrides", func(t *testing.T) {
		metaConfig := generateMetaConfigForConfigOverridesTest(t, map[string]interface{}{
			"configOverrides": `
configOverrides:
  common:
    testString: 1
`,
		})

		_, err := ConvertInitConfigurationToModuleConfigs(metaConfig, NewSchemaStore(), "Minimal", "Debug")
		require.Error(t, err)
	})
}
