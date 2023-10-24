package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func generateMetaConfigForDeckhouseConfigTest(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, configOverridesTemplate, data)
}

func TestModuleDeckhouseConfigOverridesAndMc(t *testing.T) {
	t.Run("Fail whe module config and config overrides", func(t *testing.T) {
		metaConfig := generateMetaConfigForDeckhouseConfigTest(t, map[string]interface{}{
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
			"moduleConfigs": `
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-11-22T09:12:26Z"
  generation: 1
  name: helm
  resourceVersion: "826312837"
  uid: b275a253-dcb5-4321-b0ef-8881fdc8a2a8
spec:
  enabled: false
`,
		})

		_, err := PrepareDeckhouseInstallConfig(metaConfig)
		require.Error(t, err)
	})
}
