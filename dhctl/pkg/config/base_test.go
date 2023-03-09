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
kubernetesVersion: "1.23"
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
}
