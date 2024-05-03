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

package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

func TestParseConfigFromData(t *testing.T) {
	clusterConfig := `
---
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.29"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	initConfig := `
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
   imagesRepo: test
   devBranch: test
   configOverrides: {}
`
	staticConfig := `
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.0.0/24
`
	moduleConfigGlobalValid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: false
    modules:
      publicDomainTemplate: '%s.domain.example.com'
  version: 1
`
	moduleConfigGlobalInvalid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    highAvailability: "wswswswss"
    modules:
      publicDomainTemplate: 'domain.example.com'
  version: 1
`

	moduleConfigCommonInvalid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: true
  settings:
    testString: true
    testArray: 1
    testEnum: c
  version: 1
`
	moduleConfigCommonValid := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: false
  settings:
    testString: "aaaaa"
    testArray: ["1", "2"]
    testEnum: Aa
  version: 1
`

	moduleConfigCommonWithoutEnabled := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  settings:
    testString: "aaaaa"
    testArray: ["1", "2"]
    testEnum: Aa
  version: 1
`

	moduleConfigCommonWithoutSettings := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: common
spec:
  enabled: false
`
	unknownModuleConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: unknown
spec:
  enabled: true
`

	configMapAndInstanceClass := `
---
apiVersion: v1
data:
  isUpdating: "false"
  notified: "false"
kind: ConfigMap
metadata:
  labels:
    heritage: deckhouse
  name: d8-release-data
  namespace: d8-system
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  memory: 8192
`

	ngWithTemplating := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    maxPerZone: 1
    minPerZone: 1
    zones:
    - ru-central1-a
    additionalSubnets:
    - '{{ index .cloudDiscovery.zoneToSubnetIdMap "ru-central1-a" }}'
  disruptions:
    approvalMode: Automatic
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
`

	t.Run("Standard Static", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(clusterConfig + initConfig)
		require.NoError(t, err)

		parsedStaticConfig, err := metaConfig.StaticClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedStaticConfig))

		parsedProviderConfig, err := metaConfig.ProviderClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedProviderConfig))

		require.Equal(t, "10.111.0.10", metaConfig.ClusterDNSAddress)
		require.Equal(t, "Static", metaConfig.ClusterType)

		require.Len(t, metaConfig.ResourcesYAML, 0)
	})

	t.Run("Without init configuration", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(clusterConfig)
		require.NoError(t, err)

		parsedStaticConfig, err := metaConfig.StaticClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedStaticConfig))

		parsedProviderConfig, err := metaConfig.ProviderClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedProviderConfig))

		require.Equal(t, "10.111.0.10", metaConfig.ClusterDNSAddress)
		require.Equal(t, "Static", metaConfig.ClusterType)

		require.Equal(t, metaConfig.Registry.Address, "registry.deckhouse.io")
		require.Equal(t, metaConfig.Registry.Address, "registry.deckhouse.io")
		require.Equal(t, metaConfig.Registry.Path, "/deckhouse/ce")
		require.Equal(t, metaConfig.Registry.DockerCfg, "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=")
		require.Equal(t, metaConfig.Registry.Scheme, "https")

		require.Len(t, metaConfig.ResourcesYAML, 0)
	})

	t.Run("Static with StaticClusterConfig", func(t *testing.T) {
		metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig)
		require.NoError(t, err)

		parsedStaticConfig, err := metaConfig.StaticClusterConfigYAML()
		require.NoError(t, err)
		require.YAMLEq(t, staticConfig, string(parsedStaticConfig))

		parsedProviderConfig, err := metaConfig.ProviderClusterConfigYAML()
		require.NoError(t, err)
		require.Equal(t, 0, len(parsedProviderConfig))

		require.Equal(t, "10.111.0.10", metaConfig.ClusterDNSAddress)
		require.Equal(t, "Static", metaConfig.ClusterType)

		require.Len(t, metaConfig.ResourcesYAML, 0)
	})

	t.Run("Module config", func(t *testing.T) {
		t.Run("Global valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigGlobalValid)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Global invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigGlobalInvalid)
			require.Error(t, err)
		})

		t.Run("Module valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonValid)

			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Module invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonInvalid)
			require.Error(t, err)
		})

		t.Run("Module without enabled field", func(t *testing.T) {
			_, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonWithoutEnabled)
			require.Error(t, err)
		})

		t.Run("Module without settings", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonWithoutSettings)
			require.NoError(t, err)

			require.Len(t, metaConfig.ResourcesYAML, 0)
		})

		t.Run("Unknown module should move into resources", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + unknownModuleConfig)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)
		})
	})

	t.Run("Config with another k8s resources eg configMap", func(t *testing.T) {
		t.Run("Should move another resources into resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + configMapAndInstanceClass)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)

			bigFileTmp := strings.TrimSpace(metaConfig.ResourcesYAML)
			docs := input.YAMLSplitRegexp.Split(bigFileTmp, -1)

			configMapFound := false
			instanceClassFound := false

			for _, doc := range docs {
				var index SchemaIndex
				err := yaml.Unmarshal([]byte(doc), &index)

				require.NoError(t, err)
				require.True(t, index.IsValid())
				switch index.Kind {
				case "ConfigMap":
					configMapFound = true
				case "YandexInstanceClass":
					instanceClassFound = true
				}
			}

			require.True(t, configMapFound)
			require.True(t, instanceClassFound)
		})

		t.Run("Should move resourcesYAML", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + ngWithTemplating)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 0)
			require.True(t, len(metaConfig.ResourcesYAML) > 0)

			bigFileTmp := strings.TrimSpace(metaConfig.ResourcesYAML)

			var index SchemaIndex
			err = yaml.Unmarshal([]byte(bigFileTmp), &index)

			require.NoError(t, err)
			require.True(t, index.IsValid())

			require.Equal(t, index.Kind, "NodeGroup")

		})
	})
}
