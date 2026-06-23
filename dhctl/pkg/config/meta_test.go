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
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
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
kubernetesVersion: "1.32"
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

func renderTestConfig(data map[string]any, config string) string {
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
	res := map[string]any{
		"auths": map[string]any{
			host: make(map[string]any),
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

func generateMetaConfig(t *testing.T, template string, data map[string]any, hasErr bool) *MetaConfig {
	configData := renderTestConfig(data, template)

	cfg, err := ParseConfigFromData(context.TODO(), configData, DummyPreparatorProvider(), &options.New().Global)
	f := require.NoError
	if hasErr {
		f = require.Error
	}

	f(t, err)

	return cfg
}

func generateMetaConfigForMetaConfigTest(t *testing.T, data map[string]any) *MetaConfig {
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
				"manifests": []string{
					`
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
        imagesRepo: r.example.com/test
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
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]any(nil))
	})

	t.Run("proxy config is present, httpProxy is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{
			"proxy": map[string]any{
				"httpProxy": "http://1.2.3.4",
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]any{
			"httpProxy": "http://1.2.3.4",
			"noProxy":   []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})

	t.Run("proxy config is present, httpsProxy is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{
			"proxy": map[string]any{
				"httpsProxy": "https://2.3.4.5",
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]any{
			"httpsProxy": "https://2.3.4.5",
			"noProxy":    []string{"127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})

	t.Run("proxy config is present, all options is set", func(t *testing.T) {
		cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{
			"proxy": map[string]any{
				"httpProxy":  "http://1.2.3.4",
				"httpsProxy": "https://2.3.4.5",
				"noProxy":    []string{"example.com", ".example.com"},
			},
		})

		p, err := cfg.EnrichProxyData()
		require.NoError(t, err)

		require.Equal(t, p, map[string]any{
			"httpProxy":  "http://1.2.3.4",
			"httpsProxy": "https://2.3.4.5",
			"noProxy":    []string{"example.com", ".example.com", "127.0.0.1", "169.254.169.254", "cluster.local", "10.111.0.0/16", "10.222.0.0/16"},
		})
	})
}

func TestConfigForBashibleBundleTemplateClusterMasterEndpoints(t *testing.T) {
	cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{})
	mingetPath := filepath.Join(t.TempDir(), "minget")
	require.NoError(t, os.WriteFile(mingetPath, []byte("test-minget"), 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)
	cfg.ClusterMasterEndpoints = []ClusterMasterEndpoint{
		{
			Address:                "127.0.0.1",
			KubeAPIPort:            6443,
			RPPServerPort:          5444,
			RPPBootstrapServerPort: defaultClusterMasterRPPBootstrapServerPort,
		},
	}

	data, err := cfg.ConfigForBashibleBundleTemplate("10.0.0.2")
	require.NoError(t, err)

	endpoints, ok := data["clusterMasterEndpoints"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, endpoints, 1)
	require.Equal(t, map[string]any{
		"address":                "127.0.0.1",
		"kubeApiPort":            6443,
		"rppServerPort":          5444,
		"rppBootstrapServerPort": defaultClusterMasterRPPBootstrapServerPort,
	}, endpoints[0])
	require.Equal(t, []string{"127.0.0.1:6443"}, data["clusterMasterKubeAPIEndpoints"])
	require.Equal(t, []string{"127.0.0.1:5444"}, data["clusterMasterRPPAddresses"])
	require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", defaultClusterMasterRPPBootstrapServerPort)}, data["clusterMasterRPPBootstrapAddresses"])
}

func TestConfigForBashibleBundleTemplateDefaultClusterMasterEndpoints(t *testing.T) {
	cfg := generateMetaConfigForMetaConfigTest(t, map[string]any{})
	mingetPath := filepath.Join(t.TempDir(), "minget")
	expectedMingetBytes := []byte("test-minget")
	require.NoError(t, os.WriteFile(mingetPath, expectedMingetBytes, 0o600))
	t.Setenv("DHCTL_MINGET_PATH", mingetPath)

	data, err := cfg.ConfigForBashibleBundleTemplate("10.0.0.2")
	require.NoError(t, err)

	endpoints, ok := data["clusterMasterEndpoints"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, endpoints, 1)
	require.Equal(t, map[string]any{
		"address":                "127.0.0.1",
		"rppServerPort":          5444,
		"rppBootstrapServerPort": defaultClusterMasterRPPBootstrapServerPort,
	}, endpoints[0])
	require.Empty(t, data["clusterMasterKubeAPIEndpoints"])
	require.Equal(t, []string{"127.0.0.1:5444"}, data["clusterMasterRPPAddresses"])
	require.Equal(t, []string{fmt.Sprintf("127.0.0.1:%d", defaultClusterMasterRPPBootstrapServerPort)}, data["clusterMasterRPPBootstrapAddresses"])

	mingetB64, ok := data["mingetB64"].(string)
	require.True(t, ok)
	require.NotEmpty(t, mingetB64)

	mingetBytes, err := base64.StdEncoding.DecodeString(mingetB64)
	require.NoError(t, err)
	require.Equal(t, expectedMingetBytes, mingetBytes)
}

func TestMetaConfig_DeepCopy_PreservesPrepareInputs(t *testing.T) {
	src := &MetaConfig{
		DownloadRootDir:  "/tmp/dl",
		DownloadCacheDir: "/tmp/cache",
		VersionFilePath:  "/tmp/v.yaml",
		ResourcesYAML:    "kind: X\n",
		ModuleConfigs:    []*ModuleConfig{{Spec: ModuleConfigSpec{Settings: SettingsValues{"k": "v"}}}},
		Images:           imagesDigests{"a": map[string]interface{}{"b": "c"}},
		VersionMap:       map[string]interface{}{"k": "v"},
		InstallerVersion: "1.2.3",
		ShowProgress:     true,
	}
	src.ModuleConfigs[0].SetName("x")

	cp := src.DeepCopy()

	require.Equal(t, src.DownloadRootDir, cp.DownloadRootDir)
	require.Equal(t, src.DownloadCacheDir, cp.DownloadCacheDir)
	require.Equal(t, src.VersionFilePath, cp.VersionFilePath)
	require.Equal(t, src.ResourcesYAML, cp.ResourcesYAML)
	require.Equal(t, src.InstallerVersion, cp.InstallerVersion)
	require.True(t, cp.ShowProgress)
	require.Len(t, cp.ModuleConfigs, 1)
	require.Equal(t, "x", cp.ModuleConfigs[0].GetName())
	require.Equal(t, "v", cp.VersionMap["k"])
	require.Equal(t, "c", cp.Images["a"]["b"])
}

func TestMetaConfig_DeepCopy_CloudProviderVarsIsDeep(t *testing.T) {
	src := &MetaConfig{
		CloudProviderVars: &CloudProviderVars{
			Settings:   map[string]interface{}{"k": "v"},
			NodeGroups: map[string]map[string]interface{}{"ng": {"replicas": 1}},
		},
	}
	cp := src.DeepCopy()

	cp.CloudProviderVars.Settings["k"] = "mutated"
	cp.CloudProviderVars.NodeGroups["ng"]["replicas"] = 99

	require.Equal(t, "v", src.CloudProviderVars.Settings["k"])
	require.Equal(t, 1, src.CloudProviderVars.NodeGroups["ng"]["replicas"])
}

type stubPreparator struct {
	result proto.PrepareResult
}

func (s stubPreparator) Validate(_ context.Context, _ ProviderInput) error {
	return nil
}

func (s stubPreparator) Prepare(_ context.Context, _ ProviderInput) (proto.PrepareResult, error) {
	return s.result, nil
}

func stubPreparatorProvider(s stubPreparator) MetaConfigPreparatorProvider {
	return func(_, _ string) MetaConfigPreparator { return s }
}

func TestValidateAndPrepareMetaConfig_NilProviderClusterConfig_NoPanic(t *testing.T) {
	m := &MetaConfig{
		ClusterType:           CloudClusterType,
		ProviderName:          "dvp",
		ProviderClusterConfig: nil,
	}
	prep := stubPreparator{result: proto.PrepareResult{
		ProviderClusterConfig: map[string]interface{}{"layout": "Standard"},
	}}

	out, err := validateAndPrepareMetaConfig(context.Background(), stubPreparatorProvider(prep), m)
	require.NoError(t, err)
	require.NotNil(t, out.ProviderClusterConfig)
	require.Contains(t, out.ProviderClusterConfig, "layout")
	require.Equal(t, "standard", out.Layout)
}

func TestApplyModuleConfigSettings_TakesFullModuleConfig(t *testing.T) {
	settings := SettingsValues{"masterPool": map[string]interface{}{"replicas": 3}}
	mc := &ModuleConfig{Spec: ModuleConfigSpec{Version: 2, Settings: settings}}
	mc.SetName("cloud-provider-dvp")

	m := &MetaConfig{
		ProviderName:  "dvp",
		ModuleConfigs: []*ModuleConfig{mc},
	}

	require.NoError(t, m.applyCloudProviderModuleSettings())

	require.NotNil(t, m.CloudProviderVars)
	spec, ok := m.CloudProviderVars.Settings["spec"].(map[string]interface{})
	require.True(t, ok, "expected spec object in CloudProviderVars.Settings")
	require.Equal(t, float64(2), spec["version"])
	specSettings, ok := spec["settings"].(map[string]interface{})
	require.True(t, ok)
	masterPool, ok := specSettings["masterPool"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(3), masterPool["replicas"])
}
