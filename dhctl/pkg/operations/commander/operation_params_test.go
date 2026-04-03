// Copyright 2026 Flant JSC
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

package commander

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var clusterConf = []byte(`apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: DVP
  prefix: test
podSubnetCIDR: 10.118.0.0/16
serviceSubnetCIDR: 10.228.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: "cluster.local"
`)

func TestPrepare(t *testing.T) {
	provider := []byte(`apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
sshPublicKey: |
  ssh-rsa AAAA
`)

	clusterCopy := copyBytes(clusterConf)
	providerCopy := copyBytes(provider)

	p := NewCommanderModeParams(clusterCopy, providerCopy)

	require.Equal(t, clusterConf, p.ClusterConfigurationData, "cluster conf does not changed")
	require.NotEqual(t, provider, p.ProviderClusterConfigurationData, "cluster should changed")
	require.True(t, bytes.HasSuffix(p.ProviderClusterConfigurationData, []byte("\n# comment for safe trim")), "should has comment")
}

func TestNoPrepare(t *testing.T) {
	provider := []byte(`apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
sshPublicKey: |
  ssh-rsa AAAA
`)

	clusterCopy := copyBytes(clusterConf)
	providerCopy := copyBytes(provider)

	p := NewCommanderModeParams(clusterCopy, providerCopy)

	require.Equal(t, clusterConf, p.ClusterConfigurationData, "cluster conf does not changed")
	require.Equal(t, provider, p.ProviderClusterConfigurationData, "cluster conf does not changed")
}

func copyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
