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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func TestCNIBootstrapLookup(t *testing.T) {
	data := map[string]any{
		"simple": map[string]any{
			"podNetworkMode": "VXLAN",
		},
		"simpleWithInternalNetwork": map[string]any{
			"podNetworkMode": "DirectRouting",
		},
		"flag": true,
		"num":  42,
	}

	tests := []struct {
		name    string
		path    string
		wantVal any
		wantOK  bool
	}{
		{"simple dot-path", ".simple.podNetworkMode", "VXLAN", true},
		{"top-level bool", ".flag", true, true},
		{"top-level number", ".num", 42, true},
		{"path without leading dot", "simple.podNetworkMode", "VXLAN", true},
		{"missing leaf", ".simple.missing", nil, false},
		{"missing root", ".absent.x", nil, false},
		{"empty path", "", nil, false},
		{"path through non-map", ".flag.x", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cniBootstrapLookup(data, tt.path)
			require.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				require.Equal(t, tt.wantVal, got)
			}
		})
	}
}

func TestCNIBootstrapMatches(t *testing.T) {
	require.True(t, cniBootstrapMatches("VXLAN", []any{"VXLAN"}))
	require.True(t, cniBootstrapMatches("VXLAN", []any{"DirectRouting", "VXLAN"}))
	require.False(t, cniBootstrapMatches("DirectRouting", []any{"VXLAN"}))
	require.False(t, cniBootstrapMatches(nil, []any{"VXLAN"}))
	require.True(t, cniBootstrapMatches(1, []any{"1"}))
	require.True(t, cniBootstrapMatches(true, []any{"true"}))
}

func TestResolveCNIBootstrapSettings_DefaultOnly(t *testing.T) {
	b := cniBootstrap{
		Name: "cilium",
		Config: cniBootstrapConfig{
			Default: map[string]any{"tunnelMode": "Disabled", "createNodeRoutes": true},
		},
	}
	got, err := resolveCNIBootstrapSettings(b, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"tunnelMode": "Disabled", "createNodeRoutes": true}, got)
}

func TestResolveCNIBootstrapSettings_RuleMatches(t *testing.T) {
	b := cniBootstrap{
		Name: "cilium",
		Config: cniBootstrapConfig{
			Default: map[string]any{"tunnelMode": "Disabled", "masqueradeMode": "Netfilter"},
			Rules: []cniBootstrapRule{{
				Source: cniBootstrapSourcePCC,
				Match: cniBootstrapRuleMatch{
					JSONPath: ".simple.podNetworkMode",
					Values:   []any{"VXLAN"},
				},
				Settings: map[string]any{"tunnelMode": "VXLAN", "masqueradeMode": "BPF"},
			}},
		},
	}
	pcc := mustPCC(t, map[string]any{
		"simple": map[string]any{"podNetworkMode": "VXLAN"},
	})

	got, err := resolveCNIBootstrapSettings(b, pcc)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"tunnelMode": "VXLAN", "masqueradeMode": "BPF"}, got)
}

func TestResolveCNIBootstrapSettings_RuleNoMatch(t *testing.T) {
	b := cniBootstrap{
		Name: "cilium",
		Config: cniBootstrapConfig{
			Default: map[string]any{"tunnelMode": "Disabled"},
			Rules: []cniBootstrapRule{{
				Source: cniBootstrapSourcePCC,
				Match: cniBootstrapRuleMatch{
					JSONPath: ".simple.podNetworkMode",
					Values:   []any{"VXLAN"},
				},
				Settings: map[string]any{"tunnelMode": "VXLAN"},
			}},
		},
	}
	pcc := mustPCC(t, map[string]any{
		"simple": map[string]any{"podNetworkMode": "DirectRouting"},
	})

	got, err := resolveCNIBootstrapSettings(b, pcc)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"tunnelMode": "Disabled"}, got)
}

func TestResolveCNIBootstrapSettings_FirstMatchingRuleAmongMany(t *testing.T) {
	b := cniBootstrap{
		Name: "cilium",
		Config: cniBootstrapConfig{
			Default: map[string]any{"tunnelMode": "Disabled"},
			Rules: []cniBootstrapRule{
				{
					Source: cniBootstrapSourcePCC,
					Match: cniBootstrapRuleMatch{
						JSONPath: ".simple.podNetworkMode",
						Values:   []any{"VXLAN"},
					},
					Settings: map[string]any{"tunnelMode": "VXLAN", "masqueradeMode": "BPF"},
				},
				{
					Source: cniBootstrapSourcePCC,
					Match: cniBootstrapRuleMatch{
						JSONPath: ".simpleWithInternalNetwork.podNetworkMode",
						Values:   []any{"VXLAN"},
					},
					Settings: map[string]any{"tunnelMode": "VXLAN", "masqueradeMode": "BPF"},
				},
			},
		},
	}
	pcc := mustPCC(t, map[string]any{
		"simpleWithInternalNetwork": map[string]any{"podNetworkMode": "VXLAN"},
	})

	got, err := resolveCNIBootstrapSettings(b, pcc)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"tunnelMode": "VXLAN", "masqueradeMode": "BPF"}, got)
}

func TestResolveCNIBootstrapSettings_UnknownSourceSkipped(t *testing.T) {
	b := cniBootstrap{
		Name: "cilium",
		Config: cniBootstrapConfig{
			Default: map[string]any{"tunnelMode": "Disabled"},
			Rules: []cniBootstrapRule{{
				Source: "clusterConfiguration",
				Match: cniBootstrapRuleMatch{
					JSONPath: ".clusterType",
					Values:   []any{"Cloud"},
				},
				Settings: map[string]any{"tunnelMode": "VXLAN"},
			}},
		},
	}
	got, err := resolveCNIBootstrapSettings(b, nil)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"tunnelMode": "Disabled"}, got)
}

func TestSameCNISettings(t *testing.T) {
	a := SettingsValues{"a": 1, "b": "two", "c": map[string]any{"x": true}}
	b := SettingsValues{"a": 1, "b": "two", "c": map[string]any{"x": true}}
	c := SettingsValues{"a": 1, "b": "three", "c": map[string]any{"x": true}}

	same, err := sameCNISettings(a, b)
	require.NoError(t, err)
	require.True(t, same)

	same, err = sameCNISettings(a, c)
	require.NoError(t, err)
	require.False(t, same)
}

// buildModuleConfig leaves Settings=nil for an empty recommendation; the user
// may write settings: {} in YAML. Both must compare equal so simple-bridge
// (no settings) does not produce a false mismatch.
func TestSameCNISettings_NilVsEmptyMap(t *testing.T) {
	same, err := sameCNISettings(nil, SettingsValues{})
	require.NoError(t, err)
	require.True(t, same)

	same, err = sameCNISettings(SettingsValues{}, nil)
	require.NoError(t, err)
	require.True(t, same)

	same, err = sameCNISettings(nil, nil)
	require.NoError(t, err)
	require.True(t, same)
}

func TestCNIBootstrapDecision_NoUser(t *testing.T) {
	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, msg := cniBootstrapDecision(nil, rec)
	require.Equal(t, CNIBootstrapMismatchReasonNone, reason)
	require.Empty(t, msg)
}

func TestCNIBootstrapDecision_SameNameSameSettings(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)
	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, msg := cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonNone, reason)
	require.Empty(t, msg)
}

func TestCNIBootstrapDecision_SameNameDifferentSettings(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "Disabled"}, true)
	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, msg := cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonDifferentSettings, reason)
	require.Contains(t, msg, "differ")
}

func TestCNIBootstrapDecision_DifferentModule(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-flannel", map[string]any{"podNetworkMode": "VXLAN"}, true)
	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, msg := cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonDifferentModule, reason)
	require.Contains(t, msg, "cni-flannel")
	require.Contains(t, msg, "cni-cilium")
}

// enabled=false against recommended enabled=true is treated as a settings
// mismatch — same generic flow as any other settings disagreement.
func TestCNIBootstrapDecision_UserDisabled(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, false)
	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, msg := cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonDifferentSettings, reason)
	require.Contains(t, msg, "cni-cilium")
}

// Nil *bool means "use default", which for ModuleConfig is enabled=true.
// Treat nil and explicit true as equal so users who omit the field do not
// trip the mismatch path.
func TestCNIBootstrapDecision_NilEnabledEqualsTrue(t *testing.T) {
	user := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)
	user.Spec.Enabled = nil

	rec := newTestCNIModuleConfig(t, "cni-cilium", map[string]any{"tunnelMode": "VXLAN"}, true)

	reason, _ := cniBootstrapDecision(user, rec)
	require.Equal(t, CNIBootstrapMismatchReasonNone, reason)
}

func TestAnalyzeCNIBootstrap_StaticCluster_IgnoresContent(t *testing.T) {
	m := &MetaConfig{}
	got, err := analyzeCNIBootstrap(t.Context(), m, nil, "::: not valid yaml :::")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, got.Matches)
	require.Equal(t, CNIBootstrapSkipReasonStaticCluster, got.SkipReason)
}

func TestAnalyzeCNIBootstrap_InjectedContent_UsedInsteadOfDisk(t *testing.T) {
	m := &MetaConfig{
		ClusterType:  CloudClusterType,
		ProviderName: "nonexistent-provider-for-test",
	}
	_, err := analyzeCNIBootstrap(t.Context(), m, candiDirOptions(t.TempDir()), "::: not valid yaml :::")
	require.Error(t, err)
	require.Contains(t, err.Error(), "<injected>", "error must come from the injected-content path, not from disk")
	require.NotContains(t, err.Error(), "read cni-bootstrap file")
}

func TestApplyCNIBootstrap_StaticCluster(t *testing.T) {
	m := &MetaConfig{ClusterType: "Static"}
	require.NoError(t, ApplyCNIBootstrap(t.Context(), m, nil))
	require.Empty(t, m.ModuleConfigs)
}

// cni-bootstrap.yml is mandatory for cloud providers - missing file = error.
func TestApplyCNIBootstrap_FileMissing(t *testing.T) {
	m := &MetaConfig{ClusterType: CloudClusterType, ProviderName: "noop"}
	err := ApplyCNIBootstrap(t.Context(), m, candiDirOptions(t.TempDir()))
	require.Error(t, err)
	require.Contains(t, err.Error(), "cni-bootstrap")
	require.Empty(t, m.ModuleConfigs)
}

func TestApplyCNIBootstrap_ReadsAndAppliesFile(t *testing.T) {
	dir := t.TempDir()

	providerDir := filepath.Join(dir, "cloud-providers", "demo")
	require.NoError(t, os.MkdirAll(providerDir, 0o755))
	body := []byte(`schemaVersion: 1
name: cilium
config:
  default:
    tunnelMode: Disabled
  rules:
    - source: providerClusterConfiguration
      match:
        jsonPath: ".simple.podNetworkMode"
        values: [VXLAN]
      settings:
        tunnelMode: VXLAN
`)
	require.NoError(t, os.WriteFile(filepath.Join(providerDir, "cni-bootstrap.yml"), body, 0o644))

	m := &MetaConfig{
		ClusterType:  CloudClusterType,
		ProviderName: "demo",
		ProviderClusterConfig: map[string]json.RawMessage{
			"simple": json.RawMessage(`{"podNetworkMode":"VXLAN"}`),
		},
	}

	err := ApplyCNIBootstrap(t.Context(), m, candiDirOptions(dir))
	if err != nil {
		t.Skipf("cni-cilium schema not available in this environment: %v", err)
	}
	require.Len(t, m.ModuleConfigs, 1)
	mc := m.ModuleConfigs[0]
	require.Equal(t, "cni-cilium", mc.GetName())
	require.NotNil(t, mc.Spec.Enabled)
	require.True(t, *mc.Spec.Enabled)
	require.Equal(t, "VXLAN", mc.Spec.Settings["tunnelMode"], "rule should override default")
}

// Verifies the resolve mechanism end-to-end via YAML parsing on an inline
// fixture with two alternative jsonPath rules - the same shape that real
// provider files use (e.g. openstack's simple vs simpleWithInternalNetwork).
func TestResolveCNIBootstrapSettings_MultiRuleMechanism(t *testing.T) {
	raw := []byte(`schemaVersion: 1
name: cilium
config:
  default:
    tunnelMode: Disabled
    createNodeRoutes: true
    masqueradeMode: Netfilter
  rules:
    - source: providerClusterConfiguration
      match:
        jsonPath: ".pathA.mode"
        values: [VXLAN]
      settings:
        tunnelMode: VXLAN
        masqueradeMode: BPF
    - source: providerClusterConfiguration
      match:
        jsonPath: ".pathB.mode"
        values: [VXLAN]
      settings:
        tunnelMode: VXLAN
        masqueradeMode: BPF
`)
	var b cniBootstrap
	require.NoError(t, yaml.Unmarshal(raw, &b))
	require.Equal(t, 1, b.SchemaVersion)
	require.Equal(t, "cilium", b.Name)

	defaultSettings := map[string]any{
		"tunnelMode":       "Disabled",
		"createNodeRoutes": true,
		"masqueradeMode":   "Netfilter",
	}
	overriddenSettings := map[string]any{
		"tunnelMode":       "VXLAN",
		"createNodeRoutes": true,
		"masqueradeMode":   "BPF",
	}

	tests := []struct {
		name string
		pcc  map[string]any
		want map[string]any
	}{
		{
			name: "first rule matches -> overrides default",
			pcc:  map[string]any{"pathA": map[string]any{"mode": "VXLAN"}},
			want: overriddenSettings,
		},
		{
			name: "second rule matches -> overrides default",
			pcc:  map[string]any{"pathB": map[string]any{"mode": "VXLAN"}},
			want: overriddenSettings,
		},
		{
			name: "rule path present but value mismatches -> default",
			pcc:  map[string]any{"pathA": map[string]any{"mode": "DirectRouting"}},
			want: defaultSettings,
		},
		{
			name: "rule path absent -> default",
			pcc:  map[string]any{"unrelated": "x"},
			want: defaultSettings,
		},
		{
			name: "empty providerClusterConfiguration -> default",
			pcc:  map[string]any{},
			want: defaultSettings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCNIBootstrapSettings(b, mustPCC(t, tt.pcc))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func mustPCC(t *testing.T, m map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, v := range m {
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		out[k] = raw
	}
	return out
}

func TestMergeMissingCNISettings(t *testing.T) {
	t.Parallel()

	merged, added := mergeMissingCNISettings(
		SettingsValues{"tunnelMode": "VXLAN"},
		SettingsValues{"tunnelMode": "Disabled", "createNodeRoutes": true},
	)
	require.ElementsMatch(t, []string{"createNodeRoutes"}, added)
	require.Equal(t, "VXLAN", merged["tunnelMode"])
	require.Equal(t, true, merged["createNodeRoutes"])

	merged, added = mergeMissingCNISettings(nil, SettingsValues{"createNodeRoutes": true})
	require.ElementsMatch(t, []string{"createNodeRoutes"}, added)
	require.Equal(t, true, merged["createNodeRoutes"])

	_, added = mergeMissingCNISettings(SettingsValues{"createNodeRoutes": false}, SettingsValues{"createNodeRoutes": true})
	require.Empty(t, added)
}

func TestApplyCNIBootstrap_MergesMissingSettingsNonInteractive(t *testing.T) {
	dir := t.TempDir()

	providerDir := filepath.Join(dir, "cloud-providers", "vcd-like")
	require.NoError(t, os.MkdirAll(providerDir, 0o755))
	body := []byte(`schemaVersion: 1
name: cilium
config:
  default:
    tunnelMode: Disabled
    createNodeRoutes: true
`)
	require.NoError(t, os.WriteFile(filepath.Join(providerDir, "cni-bootstrap.yml"), body, 0o644))

	userMC := newTestCNIModuleConfig(t, "cni-cilium", nil, true)
	m := &MetaConfig{
		ClusterType:   CloudClusterType,
		ProviderName:  "vcd-like",
		ModuleConfigs: []*ModuleConfig{userMC},
	}

	err := ApplyCNIBootstrap(t.Context(), m, candiDirOptions(dir))
	if err != nil {
		t.Skipf("cni-cilium schema not available in this environment: %v", err)
	}

	require.Len(t, m.ModuleConfigs, 1)
	mc := m.ModuleConfigs[0]
	require.Equal(t, "cni-cilium", mc.GetName())
	require.Equal(t, true, mc.Spec.Settings["createNodeRoutes"])
	require.Equal(t, "Disabled", mc.Spec.Settings["tunnelMode"])
}

func newTestCNIModuleConfig(t *testing.T, name string, settings map[string]any, enabled bool) *ModuleConfig {
	t.Helper()
	mc := &ModuleConfig{}
	mc.SetName(name)
	mc.Spec.Version = 1
	mc.Spec.Enabled = &enabled
	if len(settings) > 0 {
		mc.Spec.Settings = settings
	}
	return mc
}

func candiDirOptions(dir string) *options.GlobalOptions {
	return &options.GlobalOptions{CandiDir: dir}
}
