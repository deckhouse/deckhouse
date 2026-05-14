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

package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

)

func mustRawMessage(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func newMetaConfig(t *testing.T, clusterConfig map[string]json.RawMessage, moduleConfigs []*ModuleConfig) *MetaConfig {
	t.Helper()
	m := &MetaConfig{ClusterConfig: clusterConfig, ModuleConfigs: moduleConfigs}
	require.NoError(t, m.Registry.UseDefault(false))
	return m
}

func baseClusterConfig() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"kubernetesVersion":       mustRawMessage("1.31"),
		"clusterDomain":           mustRawMessage("cluster.local"),
		"serviceSubnetCIDR":       mustRawMessage("192.168.0.0/16"),
		"podSubnetCIDR":           mustRawMessage("10.244.0.0/16"),
		"podSubnetNodeCIDRPrefix": mustRawMessage("24"),
		"clusterType":             mustRawMessage("Static"),
		"encryptionAlgorithm":     mustRawMessage("RSA-4096"),
	}
}

func newModuleConfig(name string, settings SettingsValues) *ModuleConfig {
	return &ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ModuleConfigSpec{
			Version:  1,
			Settings: settings,
		},
	}
}

func TestConfigForControlPlaneTemplates_NoModuleConfig(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), nil)

	cfg, err := m.ConfigForControlPlaneTemplates("")
	require.NoError(t, err)

	require.Equal(t, "RSA-4096", cfg.ClusterConfiguration["encryptionAlgorithm"].(string))
	require.Equal(t, "1.31", cfg.ClusterConfiguration["kubernetesVersion"].(string))
	require.Empty(t, cfg.Settings)
}

func TestConfigForControlPlaneTemplates_ModuleConfigWins(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*ModuleConfig{
		newModuleConfig("control-plane-manager", SettingsValues{
			"encryptionAlgorithm": "ECDSA-P256",
			"resourcesRequests": map[string]interface{}{
				"cpu":    "500m",
				"memory": "512Mi",
			},
		}),
	})

	cfg, err := m.ConfigForControlPlaneTemplates("")
	require.NoError(t, err)

	require.Equal(t, "ECDSA-P256", cfg.Settings["encryptionAlgorithm"])
	rr, ok := cfg.Settings["resourcesRequests"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, int64(500), rr["milliCPU"])
	require.Equal(t, int64(512*1024*1024), rr["memoryBytes"])
	require.Equal(t, "RSA-4096", cfg.ClusterConfiguration["encryptionAlgorithm"].(string))
}

func TestConfigForControlPlaneTemplates_KubernetesVersionAutomatic(t *testing.T) {
	cc := baseClusterConfig()
	cc["kubernetesVersion"] = mustRawMessage("Automatic")
	m := newMetaConfig(t, cc, nil)

	cfg, err := m.ConfigForControlPlaneTemplates("")
	require.NoError(t, err)

	require.Equal(t, DefaultKubernetesVersion, cfg.ClusterConfiguration["kubernetesVersion"].(string))
}

func TestConfigForControlPlaneTemplates_PartialResourcesRequests(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*ModuleConfig{
		newModuleConfig("control-plane-manager", SettingsValues{
			"resourcesRequests": map[string]interface{}{
				"cpu": "500m",
				// memory not set
			},
		}),
	})

	cfg, err := m.ConfigForControlPlaneTemplates("")
	require.NoError(t, err)

	rr, ok := cfg.Settings["resourcesRequests"].(map[string]interface{})
	require.True(t, ok)
	require.Contains(t, rr, "milliCPU")
	require.NotContains(t, rr, "memoryBytes")
}

func TestConfigForControlPlaneTemplates_ToMap(t *testing.T) {
	m := newMetaConfig(t, baseClusterConfig(), []*ModuleConfig{
		newModuleConfig("control-plane-manager", SettingsValues{
			"resourcesRequests": map[string]interface{}{
				"cpu":    "1000m",
				"memory": "1Gi",
			},
		}),
	})

	cfg, err := m.ConfigForControlPlaneTemplates("")
	require.NoError(t, err)

	tm := cfg.ToMap()
	require.NotNil(t, tm["settings"])
	require.NotNil(t, tm["clusterConfiguration"])
	require.NotContains(t, tm, "resourcesRequestsMilliCpuControlPlane")
	require.NotContains(t, tm, "resourcesRequestsMemoryControlPlane")
}
