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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func moduleConfigWithSettings(name string, settings SettingsValues) *ModuleConfig {
	mc := &ModuleConfig{}
	mc.SetName(name)
	mc.Spec.Settings = settings
	return mc
}

func metaConfigWith(t *testing.T, clusterConfig map[string]any, moduleConfigs ...*ModuleConfig) *MetaConfig {
	t.Helper()
	m := &MetaConfig{ClusterConfig: map[string]json.RawMessage{}, ModuleConfigs: moduleConfigs}
	for key, value := range clusterConfig {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		m.ClusterConfig[key] = raw
	}
	return m
}

func TestSetting_ReadsBothCandidates(t *testing.T) {
	m := metaConfigWith(t,
		map[string]any{
			"cloud":             map[string]any{"provider": "Yandex", "prefix": "ccprefix"},
			"podSubnetCIDR":     "10.1.0.0/16",
			"kubernetesVersion": "1.33",
		},
		moduleConfigWithSettings("global", SettingsValues{"prefix": "mcprefix"}),
		moduleConfigWithSettings("control-plane-manager", SettingsValues{
			// A number here is a YAML 1.40 that would come back as "1.4": dropped, not coerced.
			"kubernetesVersion": 1.40,
			"network":           map[string]any{"podSubnetCIDR": "10.11.0.0/16"},
		}),
	)

	prefix := m.setting(clusterSettingPrefix)
	require.Equal(t, "mcprefix", prefix.fromModuleConfig)
	require.Equal(t, "ccprefix", prefix.fromClusterConfig, "a nested ClusterConfiguration field")
	require.Equal(t, "mcprefix", prefix.value(), "the ModuleConfig setting wins")

	require.Equal(t, "10.11.0.0/16", m.setting(clusterSettingPodSubnetCIDR).value())

	version := m.setting(clusterSettingKubernetesVersion)
	require.Empty(t, version.fromModuleConfig, "a non-string setting counts as absent")
	require.Equal(t, "1.33", version.value(), "the deprecated field is the fallback")

	require.Empty(t, m.setting(clusterSettingServiceSubnetCIDR).value(), "set in neither document")
	require.Empty(t, (&MetaConfig{}).setting(clusterSettingPrefix).value(), "no documents at all")
}

// The label sends the operator to the document the value came from.
func TestSetting_Label(t *testing.T) {
	m := metaConfigWith(t,
		map[string]any{"cloud": map[string]any{"prefix": "ccprefix"}},
		moduleConfigWithSettings("control-plane-manager", SettingsValues{
			"network": map[string]any{"podSubnetCIDR": "10.11.0.0/16"},
		}),
	)

	require.Equal(t, "ModuleConfig control-plane-manager spec.settings.network.podSubnetCIDR", m.setting(clusterSettingPodSubnetCIDR).label())
	require.Equal(t, "ClusterConfiguration.cloud.prefix", m.setting(clusterSettingPrefix).label())
}

// This is the switch for the day a field leaves ClusterConfiguration: with no deprecated path in the
// table the field is not read, whatever the document still carries.
func TestSetting_WithoutDeprecatedFieldIgnoresClusterConfiguration(t *testing.T) {
	original := clusterSettingSources[clusterSettingPodSubnetCIDR]
	t.Cleanup(func() { clusterSettingSources[clusterSettingPodSubnetCIDR] = original })
	removed := original
	removed.deprecatedField = nil
	clusterSettingSources[clusterSettingPodSubnetCIDR] = removed

	m := metaConfigWith(t, map[string]any{"podSubnetCIDR": "10.1.0.0/16"})

	require.Empty(t, m.Network().PodSubnetCIDR)
}

// A setting missing from the table is a programmer error: its ModuleConfig would not be read from a
// live cluster, and the resolver would silently fall back to the deprecated field.
func TestSetting_UnknownPanics(t *testing.T) {
	require.Panics(t, func() { (&MetaConfig{}).setting("apiserverCertSANs") })
}

func TestClusterSettingsModuleConfigs_DerivedFromTheTable(t *testing.T) {
	names := ClusterSettingsModuleConfigs()

	for setting, source := range clusterSettingSources {
		require.Contains(t, names, source.moduleConfig, setting)
	}
	require.IsIncreasing(t, names, "unique and in a stable order")
}
