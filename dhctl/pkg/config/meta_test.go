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
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/require"
)

func TestGetDNSAddress(t *testing.T) {
	tests := []struct {
		name   string
		cidr   string
		result string
	}{
		{
			"OK",
			"10.222.0.0/16",
			"10.222.0.10",
		},
		{
			"Bad CIDR",
			"bad cidr",
			"",
		},
		{
			"Tight Mask",
			"10.222.0.0/32",
			"",
		},
		{
			"Not from zero",
			"192.168.0.18/28",
			"192.168.0.26",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.result, getDNSAddress(testCase.cidr))
		})
	}
}

func renderTestConfig(data map[string]interface{}) string {
	config := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: cluster
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.21"
clusterDomain: "cluster.local"
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  devBranch: main
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse CE is set
  imagesRepo: {{ .imagesRepo }}
  # a special string with parameters to access Docker registry
  registryDockerCfg: {{ .dockerCfg | b64enc }}
  configOverrides:
    prometheusMadisonIntegrationEnabled: false
    global:
      modules:
        publicDomainTemplate: "%s.example.com"
    nginxIngressEnabled: false
---
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: id
    diskSizeGB: 50
    externalIPAddresses:
      - Auto
sshPublicKey: ssh-rsa AAAA
nodeNetworkCIDR: 10.100.0.0/21
provider:
  cloudID: idCloud
  folderID: idFolder
  serviceAccountJSON: |
    {
       "id": "id",
       "service_account_id": "saID",
       "created_at": "2020-01-01T00:00:00Z",
       "key_algorithm": "RSA_2048",
       "public_key": "publicKey",
       "private_key": "privateKey"
    }
`
	t := template.New("testconfig_template").Funcs(sprig.TxtFuncMap())
	t, err := t.Parse(config)
	if err != nil {
		panic(err)
	}

	var tpl bytes.Buffer

	err = t.Execute(&tpl, data)
	if err != nil {
		panic(err)
	}

	return tpl.String()
}

func generateDockerCfg() string {
	dockerCfgAuth := base64.StdEncoding.EncodeToString([]byte("a:b"))
	return fmt.Sprintf(`{ "auths": { "r.example.com": { "auth": "%s" } } }`, dockerCfgAuth)
}

func TestPrepare(t *testing.T) {
	t.Run("Registry", func(t *testing.T) {
		dataToRender := map[string]interface{}{
			"dockerCfg":  generateDockerCfg(),
			"imagesRepo": "r.example.com/deckhouse/ce/",
		}
		configData := renderTestConfig(dataToRender)

		cfg, err := ParseConfigFromData(configData)
		require.NoError(t, err)

		t.Run("Trim right slash for imagesRepo", func(t *testing.T) {
			require.Equal(t, cfg.DeckhouseConfig.ImagesRepo, "r.example.com/deckhouse/ce")
		})

		t.Run("Correct prepare registry object", func(t *testing.T) {
			expectedData := RegistryData{
				Address:   "r.example.com",
				Path:      "/deckhouse/ce",
				Scheme:    "https",
				CA:        "",
				DockerCfg: "eyAiYXV0aHMiOiB7ICJyLmV4YW1wbGUuY29tIjogeyAiYXV0aCI6ICJZVHBpIiB9IH0gfQ==",
			}

			require.Equal(t, cfg.Registry, expectedData)
		})
	})
}
