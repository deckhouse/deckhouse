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
	"context"
	"encoding/json"
	"fmt"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
)

// endOfLifeStatus is what candi/version_map.yml marks a Kubernetes version with once it is out of
// support.
const endOfLifeStatus = "end-of-life"

// warnAboutKubernetesVersion says the two things the schema cannot.
//
// The enum of kubernetesVersion lists every version this installer knows, including the ones that
// have reached end of life: version_map.yml is where that is recorded, and nothing reads it back
// to the operator. A cluster created on an end-of-life version is one that needs upgrading the
// day it is built.
//
// The second is the conflict between the two places the version can be set. The ModuleConfig
// setting wins whenever it is set, and the ClusterConfiguration field is then decorative — which
// is exactly the shape of a mistake where the operator edits the one that does nothing.
//
// Both are warnings: neither makes the configuration invalid, and converge re-reads this.
func warnAboutKubernetesVersion(ctx context.Context, m *MetaConfig) {
	declared := m.declaredKubernetesVersion()
	if declared == "" || declared == "Automatic" {
		return
	}

	if status := m.kubernetesVersionStatus(declared); status == endOfLifeStatus {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"Kubernetes %s has reached end of life in this Deckhouse release. The cluster will be created on it and "+
				"will need upgrading; consider setting kubernetesVersion to a supported version, or to Automatic",
			declared))
	}

	if moduleVersion := m.moduleKubernetesVersion(); moduleVersion != "" && moduleVersion != declared {
		dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf(
			"ClusterConfiguration.kubernetesVersion is %q and kubernetesVersion in the \"control-plane-manager\" "+
				"ModuleConfig is %q. The ModuleConfig setting is the one that decides the cluster version, so the "+
				"cluster will be created on %s",
			declared, moduleVersion, moduleVersion))
	}
}

func (m *MetaConfig) declaredKubernetesVersion() string {
	raw, ok := m.ClusterConfig["kubernetesVersion"]
	if !ok || len(raw) == 0 {
		return ""
	}
	var version string
	if err := json.Unmarshal(raw, &version); err != nil {
		return ""
	}
	return version
}

// moduleKubernetesVersion reads the setting that overrides the ClusterConfiguration field.
func (m *MetaConfig) moduleKubernetesVersion() string {
	mc := m.FindModuleConfig("control-plane-manager")
	if mc == nil {
		return ""
	}
	version, _ := mc.Spec.Settings["kubernetesVersion"].(string)
	return version
}

// kubernetesVersionStatus reads the status version_map.yml records for a version. An unknown
// version has no status, which is not something to warn about: the schema enum has already
// refused anything this installer does not know.
func (m *MetaConfig) kubernetesVersionStatus(version string) string {
	versions, ok := m.VersionMap["k8s"].(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := versions[version].(map[string]any)
	if !ok {
		return ""
	}
	status, _ := entry["status"].(string)
	return status
}
