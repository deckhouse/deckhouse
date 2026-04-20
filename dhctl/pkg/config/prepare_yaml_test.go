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

package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoPrepareProviderYAML(t *testing.T) {
	generateDoc := func(kind string) []byte {
		return []byte(fmt.Sprintf(`kind: %s
layout: Standard
provider: 
  key: key
  val: value
sshPublicKey: |
  ssh-rsa AAAAAAA
`, kind))
	}

	kinds := []string{
		"StaticClusterConfiguration",
		"DynamixClusterConfiguration",
		"HuaweiCloudClusterConfiguration",
		"OpenStackClusterConfiguration",
		"VCDClusterConfiguration",
		"VsphereClusterConfiguration",
		"ZvirtClusterConfiguration",
		"AWSClusterConfiguration",
		"AzureClusterConfiguration",
		"GCPClusterConfiguration",
		"YandexClusterConfiguration",
	}

	assertNotChange := func(t *testing.T, kind string, provider []byte) {
		copyProvider := copyBytes(provider)

		res := PrepareProviderConfigYAML(provider)

		require.Equal(t, copyProvider, res, "provider conf should not changed")
	}

	for _, k := range kinds {
		t.Run(fmt.Sprintf("kind %s", k), func(t *testing.T) {
			provider := generateDoc(k)
			assertNotChange(t, k, provider)
		})
	}

	t.Run("Not provider config", func(t *testing.T) {
		doc := []byte(`kind: ModuleConfig
enabled: true
version: 1
settings:
  set: val
`)
		assertNotChange(t, "ModuleConfig", doc)
	})

	t.Run("No kind", func(t *testing.T) {
		doc := []byte(`enabled: true
version: 1
settings:
  set: val
`)
		assertNotChange(t, "", doc)
	})

	t.Run("Invalid yaml", func(t *testing.T) {
		doc := []byte(`3"rfrf!`)
		assertNotChange(t, "", doc)
	})
}

func TestPrepare(t *testing.T) {
	additionalSchemasDirs := prepareLocalRunForPrepareYAML(t)

	providerConfig := func(append string) string {
		base := `apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
masterNodeGroup:
  replicas: 1
  instanceClass:
    virtualMachine:
      cpu:
        cores: 4
        coreFraction: 100%
      memory:
        size: 8Gi
      virtualMachineClassName: "amd-epyc-gen-3"
      ipAddresses:
        - Auto
    rootDisk:
      # Root disk size.
      size: 50Gi
      storageClass: local
      image:
        kind: ClusterVirtualImage
        name: ubuntu-24-04-lts
    etcdDisk:
      size: 15Gi
      storageClass: local
`
		return base + append
	}

	type providerSettings struct {
		KubeConfig string `json:"kubeconfigDataBase64,omitempty"`
		Namespace  string `json:"namespace,omitempty"`
	}

	schemaStore := newSchemaStore(nil, additionalSchemasDirs)

	extractSettings := func(t *testing.T, doc []byte) (string, providerSettings) {
		metaCfg := &MetaConfig{}
		found, err := parseDocument(string(doc), metaCfg, schemaStore)

		require.True(t, found, "should found document")
		require.NoError(t, err, "should parse document")

		var sshKey string
		err = json.Unmarshal(metaCfg.ProviderClusterConfig["sshPublicKey"], &sshKey)
		require.NoError(t, err, "should extract ssh public key")

		provider := providerSettings{}
		err = json.Unmarshal(metaCfg.ProviderClusterConfig["provider"], &provider)
		require.NoError(t, err, "should extract ssh public key")

		return sshKey, provider
	}

	type prepareTest struct {
		name           string
		content        string
		expectedSSHKey string
		prepare        bool
	}

	tests := []prepareTest{
		{
			name: "one string key middle",
			content: providerConfig(`sshPublicKey: "ssh-rsa AAAA"
provider:
  kubeconfigDataBase64: YXB
  namespace: test
`),
			expectedSSHKey: "ssh-rsa AAAA",
			prepare:        false,
		},
		{
			name: "one string key in the end no new line",
			content: providerConfig(`provider:
  kubeconfigDataBase64: YXB
  namespace: test
sshPublicKey: ssh-rsa AAAB`),
			expectedSSHKey: "ssh-rsa AAAB",
			prepare:        false,
		},
		{
			name: "one string key in the end with new line",
			content: providerConfig(`provider:
  kubeconfigDataBase64: YXB
  namespace: test
sshPublicKey: "ssh-rsa AAAC"
`),
			expectedSSHKey: "ssh-rsa AAAC",
			prepare:        false,
		},
		{
			name: "multiline string key middle",
			content: providerConfig(`sshPublicKey: |
  ssh-rsa AAAD
provider:
  kubeconfigDataBase64: YXB
  namespace: test
`),
			expectedSSHKey: "ssh-rsa AAAD\n",
			prepare:        true,
		},
		{
			name: "multiline string key in the end no new line",
			content: providerConfig(`provider:
  kubeconfigDataBase64: YXB
  namespace: test
sshPublicKey: |
  ssh-rsa AAAE`),
			expectedSSHKey: "ssh-rsa AAAE",
			prepare:        false,
		},
		{
			name: "multiline string key in the end with new line",
			content: providerConfig(`provider:
  kubeconfigDataBase64: YXB
  namespace: test
sshPublicKey: |
  ssh-rsa AAAF
`),
			expectedSSHKey: "ssh-rsa AAAF\n",
			prepare:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentBytes := []byte(tt.content)
			contentBytesCopy := copyBytes(contentBytes)

			res := PrepareProviderConfigYAML(contentBytes)

			if tt.prepare {
				require.NotEqual(t, contentBytesCopy, res)
				require.True(t, bytes.HasSuffix(res, []byte("\n# comment for safe trim")))
			} else {
				require.Equal(t, contentBytesCopy, res)
			}

			sshKeyInDoc, provider := extractSettings(t, res)

			require.Equal(t, tt.expectedSSHKey, sshKeyInDoc, "ssh key should equal")
			require.Equal(t, "YXB", provider.KubeConfig, "kube config should equal")
			require.Equal(t, "test", provider.Namespace, "namespace should equal")
		})
	}
}

func prepareLocalRunForPrepareYAML(t *testing.T) []string {
	const cloudProvidersDir = "/deckhouse/candi/cloud-providers"

	stat, err := os.Stat(cloudProvidersDir)
	if err == nil {
		require.True(t, stat.IsDir(), "should be directory %s", cloudProvidersDir)
		return []string{cloudProvidersDir}
	}

	return []string{"/deckhouse/modules/030-cloud-provider-dvp/candi"}
}

func copyBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
