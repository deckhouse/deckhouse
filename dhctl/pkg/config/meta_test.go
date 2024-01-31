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
	"encoding/json"
	"fmt"

	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

const metaConfigTestsTemplate = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Yandex
  prefix: cluster
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.29"
clusterDomain: "cluster.local"
{{- if .proxy }}
proxy:
  {{- if .proxy.httpProxy }}
  httpProxy: {{ .proxy.httpProxy }}
  {{- end }}
  {{- if .proxy.httpsProxy }}
  httpsProxy: {{ .proxy.httpsProxy }}
  {{- end }}
  {{- if .proxy.noProxy }}
  noProxy:
    {{- range .proxy.noProxy }}
    - {{ . }}
    {{- end }}
  {{- end }}
{{- end }}
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  releaseChannel: Stable
  # address of the registry where the installer image is located; in this case, the default value for Deckhouse CE is set
{{- if .imagesRepo }}
  imagesRepo: {{ .imagesRepo }}
{{- end }}

{{- if .dockerCfg }}
  # a special string with parameters to access Docker registry
  registryDockerCfg: {{ .dockerCfg | b64enc }}
{{- end }}
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
---
`

func renderTestConfig(data map[string]interface{}, config string) string {
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

func dockerCfgAuth(username, password string) string {
	auth := fmt.Sprintf("%s:%s", username, password)
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func generateDockerCfg(host, username, password string) string {
	return fmt.Sprintf(`{"auths":{"%s":{"auth":"%s"}}}`, host, dockerCfgAuth(username, password))
}

func generateOldDockerCfg(host string, username, password *string) string {
	res := map[string]interface{}{
		"auths": map[string]interface{}{
			host: make(map[string]interface{}),
		},
	}

	if username != nil {
		err := unstructured.SetNestedField(res, *username, "auths", host, "username")
		if err != nil {
			panic(err)
		}
	}

	if password != nil {
		err := unstructured.SetNestedField(res, *password, "auths", host, "password")
		if err != nil {
			panic(err)
		}
	}

	auth, err := json.Marshal(res)
	if err != nil {
		panic(err)
	}

	return string(auth)
}

func generateMetaConfig(t *testing.T, template string, data map[string]interface{}, hasErr bool) *MetaConfig {
	configData := renderTestConfig(data, template)

	cfg, err := ParseConfigFromData(configData)
	f := require.NoError
	if hasErr {
		f = require.Error
	}

	f(t, err)

	return cfg
}

func generateMetaConfigForMetaConfigTest(t *testing.T, data map[string]interface{}) *MetaConfig {
	return generateMetaConfig(t, metaConfigTestsTemplate, data, false)
}

func TestPrepareRegistry(t *testing.T) {
	t.Run("Has imagesRepo and dockerCfg", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
			"dockerCfg":  generateDockerCfg("r.example.com", "a", "b"),
			"imagesRepo": "r.example.com/deckhouse/ce/",
		})

		t.Run("Trim right slash for imagesRepo", func(t *testing.T) {
			require.Equal(t, cfg.DeckhouseConfig.ImagesRepo, "r.example.com/deckhouse/ce")
		})

		t.Run("Correct prepare registry object", func(t *testing.T) {
			expectedData := RegistryData{
				Address:   "r.example.com",
				Path:      "/deckhouse/ce",
				Scheme:    "https",
				CA:        "",
				DockerCfg: "eyJhdXRocyI6eyJyLmV4YW1wbGUuY29tIjp7ImF1dGgiOiJZVHBpIn19fQ==",
			}

			require.Equal(t, cfg.Registry, expectedData)
		})
	})

	t.Run("Has not imagesRepo and dockerCfg", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, make(map[string]interface{}))

		t.Run("Registry object for CE edition", func(t *testing.T) {
			expectedData := RegistryData{
				Address:   "registry.deckhouse.io",
				Path:      "/deckhouse/ce",
				Scheme:    "https",
				CA:        "",
				DockerCfg: "eyJhdXRocyI6IHsgInJlZ2lzdHJ5LmRlY2tob3VzZS5pbyI6IHt9fX0=",
			}

			require.Equal(t, cfg.Registry, expectedData)
		})
	})

	t.Run("Validate registryDockerCfg", func(t *testing.T) {
		t.Run("Expect successful validation", func(t *testing.T) {
			creds := []string{
				`{"auths": { "registry.deckhouse.io": {}}}`,
				`{"auths": { "regi-stry.deckhouse.io": {}}}`,
				`{"auths": { "registry.io": {}}}`,
				`{"auths": { "1.io": {}}}`,
				`{"auths": { "1.s.io": {}}}`,
				`{"auths": { "regi.stry:5000": {}}}`,
				`{"auths": { "1.2.3": {}}}`,
				`{"auths": { "1.2:5000": {}}}`,
				`{"auths": { "reg.dec.io1": {}}}`,
				`{"auths": { "one.two.three.four.five.six.whatever": {}}}`,
				`{"auths": { "1.2.3.4.5.6.0": {}}}`,
				``,
			}

			for _, cred := range creds {
				dockerCfg := base64.StdEncoding.EncodeToString([]byte(cred))

				err := validateRegistryDockerCfg(dockerCfg)
				require.NoError(t, err)
			}
		})

		t.Run("Expect failed validation", func(t *testing.T) {
			hosts := []string{
				"some-bad-host:1434/deckhouse",
				"some-bad/deckhouse",
				".some-bad/deckhouse",
				"-some.bad",
				"somebad.",
				"some--ba",
				"some..ba",
				"14214.ba1::1554",
				"some.bad:host",
				"some-bad:host1",
			}

			for _, host := range hosts {
				creds := fmt.Sprintf("{\"auths\": { \"%s\": {}}}", host)
				dockerCfg := base64.StdEncoding.EncodeToString([]byte(creds))

				err := validateRegistryDockerCfg(dockerCfg)
				require.EqualErrorf(t,
					err,
					fmt.Sprintf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", host),
					err.Error())
			}
		})
	})
}

func TestParseRegistryData(t *testing.T) {
	t.Run("dockerCfg in current format (has auth)", func(t *testing.T) {
		t.Run("sets auth key from auth string", func(t *testing.T) {
			user, password := "user", "password"
			cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
				"dockerCfg":  generateDockerCfg("r.example.com", user, password),
				"imagesRepo": "r.example.com/deckhouse/ce/",
			})

			m, err := cfg.ParseRegistryData()
			require.NoError(t, err)

			require.Equal(t, m["auth"], dockerCfgAuth(user, password))
		})
	})

	t.Run("dockerCfg in old format (has username and password)", func(t *testing.T) {
		t.Run("correct", func(t *testing.T) {
			t.Run("sets auth key as base64 concatenation username and password with ':' separator", func(t *testing.T) {
				user, password := "old_user", "old_password"
				cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
					"dockerCfg":  generateOldDockerCfg("r.example.com", &user, &password),
					"imagesRepo": "r.example.com/deckhouse/ce/",
				})

				m, err := cfg.ParseRegistryData()
				require.NoError(t, err)

				require.Equal(t, m["auth"], dockerCfgAuth(user, password))
			})
		})

		t.Run("does not have username", func(t *testing.T) {
			t.Run("sets empty auth key", func(t *testing.T) {
				password := "old_password"
				cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
					"dockerCfg":  generateOldDockerCfg("r.example.com", nil, &password),
					"imagesRepo": "r.example.com/deckhouse/ce/",
				})

				m, err := cfg.ParseRegistryData()
				require.NoError(t, err)

				require.Equal(t, m["auth"], "")
			})
		})

		t.Run("does not have password", func(t *testing.T) {
			t.Run("sets empty auth key", func(t *testing.T) {
				user := "old_user"
				cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
					"dockerCfg":  generateOldDockerCfg("r.example.com", &user, nil),
					"imagesRepo": "r.example.com/deckhouse/ce/",
				})

				m, err := cfg.ParseRegistryData()
				require.NoError(t, err)

				require.Equal(t, m["auth"], "")
			})
		})
	})

	t.Run("default dockerCfg", func(t *testing.T) {
		t.Run("sets empty auth key", func(t *testing.T) {
			cfg := generateMetaConfigForMetaConfigTest(t, make(map[string]interface{}))

			m, err := cfg.ParseRegistryData()
			require.NoError(t, err)

			require.Equal(t, m["auth"], "")
		})
	})
}

func TestEnrichProxyData(t *testing.T) {
	t.Run("proxy config is absent", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]interface{}(nil))
	})

	t.Run("proxy config is present, httpProxy is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
			"proxy": map[string]interface{}{
				"httpProxy": "http://1.2.3.4",
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]interface{}{
			"httpProxy": "http://1.2.3.4",
			"noProxy":   []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})

	t.Run("proxy config is present, httpsProxy is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
			"proxy": map[string]interface{}{
				"httpsProxy": "https://2.3.4.5",
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]interface{}{
			"httpsProxy": "https://2.3.4.5",
			"noProxy":    []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})

	t.Run("proxy config is present, all options is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]interface{}{
			"proxy": map[string]interface{}{
				"httpProxy":  "http://1.2.3.4",
				"httpsProxy": "https://2.3.4.5",
				"noProxy":    []string{"example.com", ".example.com"},
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]interface{}{
			"httpProxy":  "http://1.2.3.4",
			"httpsProxy": "https://2.3.4.5",
			"noProxy":    []string{"example.com", ".example.com", "127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})
}
