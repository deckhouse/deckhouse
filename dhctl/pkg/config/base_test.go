package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfigFromData(t *testing.T) {
	clusterConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Static
kubernetesVersion: "1.16"
podSubnetCIDR: 10.222.0.0/16
serviceSubnetCIDR: 10.111.0.0/16
`
	initConfig := `
---
apiVersion: deckhouse.io/v1alpha1
kind: InitConfiguration
deckhouse:
   imagesRepo: test
   devBranch: test
   configOverrides: {}
`
	staticConfig := `
---
apiVersion: deckhouse.io/v1alpha1
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
