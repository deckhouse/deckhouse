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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
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
kubernetesVersion: "1.30"
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
{{- with .initConfiguration }}
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
	{{- with .imagesRepo }}
  imagesRepo: {{ . }}
	{{- end }}
	{{- with .registryDockerCfg }}
  registryDockerCfg: {{ . | b64enc }}
	{{- end }}
{{- end }}
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  enabled: true
  settings:
    modules:
      publicDomainTemplate: "%s.example.com"
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
nodeGroups:
  - name: node-group-1
    replicas: 2
    instanceClass:
      cores: 4
      memory: 8192
      imageID: id
      diskSizeGB: 50
      externalIPAddresses:
        - Auto
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
{{- with .manifests }}
	{{- range . }}
---
		{{- . }}
	{{- end }}
{{- end }}
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

	cfg, err := ParseConfigFromData(context.TODO(), configData, DummyPreparatorProvider())
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

// Registry
func TestPrepareRegistry(t *testing.T) {
	t.Run("With CRI (module enable)", func(t *testing.T) {
		t.Run("InitConfig -> unmanaged && legacy", func(t *testing.T) {
			cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{
				"initConfiguration": map[string]any{
					"imagesRepo":        "r.example.com/test/",
					"registryDockerCfg": generateDockerCfg("r.example.com", "a", "b"),
				},
			})
			require.Equal(t, true, cfg.Registry.LegacyMode)
			require.Equal(t, registry_const.ModeUnmanaged, cfg.Registry.Settings.Mode)
			registry := cfg.Registry.Settings.RemoteData
			require.Equal(t, "r.example.com/test", registry.ImagesRepo)
			require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
			require.Equal(t, "a", registry.Username)
			require.Equal(t, "b", registry.Password)
			require.Equal(t, "", registry.CA)
		})
		t.Run("Default -> CE edition registry && direct && not legacy", func(t *testing.T) {
			cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{})
			require.Equal(t, false, cfg.Registry.LegacyMode)
			require.Equal(t, registry_const.ModeDirect, cfg.Registry.Settings.Mode)
			registry := cfg.Registry.Settings.RemoteData
			require.Equal(t, "registry.deckhouse.io/deckhouse/ce", registry.ImagesRepo)
			require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
			require.Equal(t, "", registry.Password)
			require.Equal(t, "", registry.Username)
			require.Equal(t, "", registry.CA)
		})
		t.Run("ModuleConfig Deckhouse -> from moduleConfig && not legacy", func(t *testing.T) {
			cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{
				"manifests": []string{`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  settings:
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: r.example.com/test/
        username: test-user
        password: test-password
        scheme: HTTPS
        ca: "-----BEGIN CERTIFICATE-----"
  version: 1
`,
				},
			})
			require.Equal(t, false, cfg.Registry.LegacyMode)
			require.Equal(t, registry_const.ModeUnmanaged, cfg.Registry.Settings.Mode)
			registry := cfg.Registry.Settings.RemoteData
			require.Equal(t, "r.example.com/test", registry.ImagesRepo)
			require.Equal(t, registry_const.SchemeHTTPS, registry.Scheme)
			require.Equal(t, "test-user", registry.Username)
			require.Equal(t, "test-password", registry.Password)
			require.Equal(t, "-----BEGIN CERTIFICATE-----", registry.CA)
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
