// Copyright 2025 Flant JSC
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

package controlplane

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

func TestConfigForControlPlaneTemplates_NoModuleConfig(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), nil)

	cfg := getTestTemplateConfig(t, m, "")

	require.Equal(t, "RSA-4096", cfg.ClusterConfiguration["encryptionAlgorithm"].(string))
	require.Equal(t, "1.32", cfg.ClusterConfiguration["kubernetesVersion"].(string))
	require.Empty(t, cfg.Settings)
}

func TestConfigForControlPlaneTemplates_ModuleConfigWins(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*config.ModuleConfig{
		newModuleConfig("control-plane-manager", config.SettingsValues{
			"encryptionAlgorithm": "ECDSA-P256",
			"resourcesRequests": map[string]any{
				"cpu":    "500m",
				"memory": "512Mi",
			},
		}),
	})

	cfg := getTestTemplateConfig(t, m, "")

	require.Equal(t, "ECDSA-P256", cfg.Settings["encryptionAlgorithm"])
	rr, ok := cfg.Settings["resourcesRequests"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, int64(500), rr["milliCPU"])
	require.Equal(t, int64(512*1024*1024), rr["memoryBytes"])
	require.Equal(t, "RSA-4096", cfg.ClusterConfiguration["encryptionAlgorithm"].(string))
}

func TestConfigForControlPlaneTemplates_KubernetesVersionAutomatic(t *testing.T) {
	cc := baseClusterConfig()
	cc["kubernetesVersion"] = mustRawMessage("Automatic")
	m := newMetaConfig(t, cc, nil)

	cfg := getTestTemplateConfig(t, m, "")

	require.Equal(t, config.DefaultKubernetesVersion, cfg.ClusterConfiguration["kubernetesVersion"].(string))
}

func TestConfigForControlPlaneTemplates_PartialResourcesRequests(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*config.ModuleConfig{
		newModuleConfig("control-plane-manager", config.SettingsValues{
			"resourcesRequests": map[string]interface{}{
				"cpu": "500m",
				// memory not set
			},
		}),
	})

	cfg := getTestTemplateConfig(t, m, "")

	rr, ok := cfg.Settings["resourcesRequests"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, rr, "milliCPU")
	require.NotContains(t, rr, "memoryBytes")
}

func TestConfigForControlPlaneTemplates_ToMap(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*config.ModuleConfig{
		newModuleConfig("control-plane-manager", config.SettingsValues{
			"resourcesRequests": map[string]interface{}{
				"cpu":    "1000m",
				"memory": "1Gi",
			},
		}),
	})

	cfg := getTestTemplateConfig(t, m, "")

	tm := cfg.ToMap()
	require.NotNil(t, tm["settings"])
	require.NotNil(t, tm["clusterConfiguration"])
	require.Nil(t, tm["apiserver"])
}

func TestConfigForControlPlaneTemplatesAPIServerSign(t *testing.T) {
	cseSpec := "/deckhouse/ee/cse/modules/040-control-plane-manager/openapi/config-values.yaml"
	if _, err := os.Stat(cseSpec); err != nil {
		t.Skip("TestConfigForControlPlaneTemplatesAPIServerSign available only local run now")
	}

	m := newMetaConfig(t, baseClusterConfig(), []*config.ModuleConfig{
		newModuleConfig("control-plane-manager", config.SettingsValues{
			"apiserver": map[string]interface{}{
				"signature": "Enforce",
			},
		}),
	})

	cfg := getTestTemplateConfigWithEdition(
		t,
		m,
		"cse",
		cseSpec,
		"",
	)

	tm := cfg.ToMap()
	require.NotNil(t, tm["settings"])
	require.NotNil(t, tm["clusterConfiguration"])

	require.NotNil(t, tm["apiserver"], "apiserver settings should not be nil")
	apiServer, ok := tm["apiserver"].(map[string]any)
	require.True(t, ok, "apiserver should be map")
	require.Contains(t, apiServer, "signature", "apiserver settings should contains sign")
	sign, ok := apiServer["signature"].(string)
	require.True(t, ok, "sign should be string")
	require.Equal(t, sign, "Enforce", "apiserver.signature should contains valid sign mode")
}

func mustRawMessage(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func newMetaConfig(t *testing.T, clusterConfig map[string]json.RawMessage, moduleConfigs []*config.ModuleConfig) *config.MetaConfig {
	t.Helper()
	m := &config.MetaConfig{ClusterConfig: clusterConfig, ModuleConfigs: moduleConfigs}
	// Empty CRI is unsupported, so ConfigProvider falls back to legacy Unmanaged
	// mode with default registry parameters — same path the un-exported
	// useDefault(false) used to take directly.
	cfg, err := registry.NewConfigProvider(nil, nil).Config("", true)
	require.NoError(t, err)
	m.Registry = cfg
	return m
}

func baseClusterConfig() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"kubernetesVersion":       mustRawMessage("1.32"),
		"clusterDomain":           mustRawMessage("cluster.local"),
		"serviceSubnetCIDR":       mustRawMessage("192.168.0.0/16"),
		"podSubnetCIDR":           mustRawMessage("10.244.0.0/16"),
		"podSubnetNodeCIDRPrefix": mustRawMessage("24"),
		"clusterType":             mustRawMessage("Static"),
		"encryptionAlgorithm":     mustRawMessage("RSA-4096"),
	}
}

func newModuleConfig(name string, settings config.SettingsValues) *config.ModuleConfig {
	return &config.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: config.ModuleConfigSpec{
			Version:  1,
			Settings: settings,
		},
	}
}

func getTestTemplateConfig(t *testing.T, c *config.MetaConfig, nodeIP string) *TemplateConfig {
	return getTestTemplateConfigWithEdition(
		t,
		c,
		"ce",
		"/deckhouse/modules/040-control-plane-manager/openapi/config-values.yaml",
		nodeIP,
	)
}

func getTestTemplateConfigWithEdition(t *testing.T, c *config.MetaConfig, edition, spec, nodeIP string) *TemplateConfig {
	store := newTestSchemaStore(spec)
	extractor := NewSettingsExtractor(c, store, edition, dhlog.FromContext(t.Context()))

	r, err := extractor.TemplateConfigForBootstrap(nodeIP)
	require.NoError(t, err, "should return template config")
	return r
}
