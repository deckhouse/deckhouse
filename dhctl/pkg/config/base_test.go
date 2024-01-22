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
	"testing"

	"github.com/stretchr/testify/require"
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
	})

	t.Run("Module config", func(t *testing.T) {
		t.Run("Global valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigGlobalValid)
			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)
		})

		t.Run("Global invalid", func(t *testing.T) {
			_, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigGlobalInvalid)
			require.Error(t, err)
		})

		t.Run("Module valid", func(t *testing.T) {
			metaConfig, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonValid)

			require.NoError(t, err)

			require.Len(t, metaConfig.ModuleConfigs, 1)
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
			_, err := ParseConfigFromData(clusterConfig + initConfig + staticConfig + moduleConfigCommonWithoutSettings)
			require.NoError(t, err)
		})
	})
}
