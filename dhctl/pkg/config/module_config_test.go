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

		mcs, err := ConvertInitConfigurationToModuleConfigs(metaConfig)
		require.NoError(t, err)

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
				assertModuleConfig(t, mc, true, 2, map[string]interface{}{
					"testString": "aaaaa",
				})

			}
		}
	})

	t.Run("Invalid overrides", func(t *testing.T) {
		metaConfig := generateMetaConfigForConfigOverridesTest(t, map[string]interface{}{
			"configOverrides": `
configOverrides:
  common:
    testString: 1
`,
		})

		_, err := ConvertInitConfigurationToModuleConfigs(metaConfig)
		require.Error(t, err)
	})
}
