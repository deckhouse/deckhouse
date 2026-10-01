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
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// clusterSetting is a cluster-wide setting dhctl reads from a ModuleConfig.
type clusterSetting string

const (
	clusterSettingPrefix                  clusterSetting = "prefix"
	clusterSettingKubernetesVersion       clusterSetting = "kubernetesVersion"
	clusterSettingPodSubnetCIDR           clusterSetting = "podSubnetCIDR"
	clusterSettingServiceSubnetCIDR       clusterSetting = "serviceSubnetCIDR"
	clusterSettingPodSubnetNodeCIDRPrefix clusterSetting = "podSubnetNodeCIDRPrefix"
	clusterSettingClusterDomain           clusterSetting = "clusterDomain"
	clusterSettingEncryptionAlgorithm     clusterSetting = "encryptionAlgorithm"
	clusterSettingPublicDomainTemplate    clusterSetting = "publicDomainTemplate"
)

// settingSource says where a setting lives: spec.settings.<path> of ModuleConfig <moduleConfig>.
// deprecatedField is the ClusterConfiguration path it moved from, read as a fallback. Empty it when
// the field leaves ClusterConfiguration; drop the column when ClusterConfiguration is gone.
type settingSource struct {
	moduleConfig    string
	path            []string
	deprecatedField []string
}

// clusterSettingSources is the one list of them. dhctl reads the ModuleConfigs named here from a live
// cluster (ClusterSettingsModuleConfigs), so a new row reaches the CLI and the commander path alike.
var clusterSettingSources = map[clusterSetting]settingSource{
	clusterSettingPrefix:                  {"global", []string{"prefix"}, []string{"cloud", "prefix"}},
	clusterSettingKubernetesVersion:       {"control-plane-manager", []string{"kubernetesVersion"}, []string{"kubernetesVersion"}},
	clusterSettingPodSubnetCIDR:           {"control-plane-manager", []string{"network", "podSubnetCIDR"}, []string{"podSubnetCIDR"}},
	clusterSettingServiceSubnetCIDR:       {"control-plane-manager", []string{"network", "serviceSubnetCIDR"}, []string{"serviceSubnetCIDR"}},
	clusterSettingPodSubnetNodeCIDRPrefix: {"control-plane-manager", []string{"network", "podSubnetNodeCIDRPrefix"}, []string{"podSubnetNodeCIDRPrefix"}},
	clusterSettingClusterDomain:           {"control-plane-manager", []string{"network", "clusterDomain"}, []string{"clusterDomain"}},
	clusterSettingEncryptionAlgorithm:     {"control-plane-manager", []string{"encryptionAlgorithm"}, []string{"encryptionAlgorithm"}},
	clusterSettingPublicDomainTemplate:    {"global", []string{"modules", "publicDomainTemplate"}, nil},
}

// settingCandidates are the two values one setting can have. A value that is absent or not a string
// is empty: a YAML 1.40 would otherwise be coerced to "1.4".
type settingCandidates struct {
	source            settingSource
	fromModuleConfig  string
	fromClusterConfig string
}

// setting reads both candidates. spec.settings is read as written: no settings-version conversion
// runs in dhctl, so a conversion of a listed key must be reflected in clusterSettingSources.
func (m *MetaConfig) setting(name clusterSetting) settingCandidates {
	source, ok := clusterSettingSources[name]
	if !ok {
		panic(fmt.Sprintf("cluster setting %q is not in clusterSettingSources", name))
	}

	return settingCandidates{
		source:            source,
		fromModuleConfig:  m.moduleConfigString(source.moduleConfig, source.path),
		fromClusterConfig: m.clusterConfigString(source.deprecatedField),
	}
}

// value is the ModuleConfig setting when it is set, else the deprecated ClusterConfiguration field.
func (c settingCandidates) value() string {
	if c.fromModuleConfig != "" {
		return c.fromModuleConfig
	}
	return c.fromClusterConfig
}

// label names the document the value came from, so an error sends the operator to the right one.
func (c settingCandidates) label() string {
	if c.fromModuleConfig != "" {
		return "ModuleConfig " + c.source.moduleConfig + " spec.settings." + strings.Join(c.source.path, ".")
	}
	return "ClusterConfiguration." + strings.Join(c.source.deprecatedField, ".")
}

func (m *MetaConfig) moduleConfigString(moduleConfig string, path []string) string {
	mc := m.FindModuleConfig(moduleConfig)
	if mc == nil {
		return ""
	}

	value, _, err := unstructured.NestedString(mc.Spec.Settings, path...)
	if err != nil {
		return ""
	}
	return value
}

func (m *MetaConfig) clusterConfigString(path []string) string {
	if len(path) == 0 {
		return ""
	}

	fields := m.ClusterConfig
	for _, key := range path[:len(path)-1] {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(fields[key], &nested); err != nil {
			return ""
		}
		fields = nested
	}

	var value string
	if err := json.Unmarshal(fields[path[len(path)-1]], &value); err != nil {
		return ""
	}
	return value
}

// EncryptionAlgorithm is the algorithm the cluster pins for its keys. Empty means the library default.
func (m *MetaConfig) EncryptionAlgorithm() string {
	return m.setting(clusterSettingEncryptionAlgorithm).value()
}

// ClusterSettingsModuleConfigs names the ModuleConfigs clusterSettingSources refers to. In-cluster
// dhctl needs get on each of them (modules/040-terraform-manager/templates/*/rbac-for-us.yaml).
func ClusterSettingsModuleConfigs() []string {
	names := make([]string, 0, len(clusterSettingSources))
	for _, source := range clusterSettingSources {
		if !slices.Contains(names, source.moduleConfig) {
			names = append(names, source.moduleConfig)
		}
	}
	slices.Sort(names)
	return names
}
