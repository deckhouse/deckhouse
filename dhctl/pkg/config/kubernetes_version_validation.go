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
	"fmt"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"
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
	version := m.setting(clusterSettingKubernetesVersion)
	declared, moduleVersion := version.fromClusterConfig, version.fromModuleConfig
	if declared == "" || declared == "Automatic" {
		return
	}

	if status := m.kubernetesVersionStatus(declared); status == endOfLifeStatus {
		dhlog.FromContext(ctx).WarnContext(ctx, configurationFailure(
			"ClusterConfiguration.kubernetesVersion",
			fmt.Sprintf("%s, marked end-of-life in this Deckhouse release", declared),
			"a version that is still supported",
			"set kubernetesVersion to a supported version, or to Automatic").Error())
	}

	// "Will be created" holds only before the cluster exists. On a live one the deprecation notice of
	// the field says the same, and check or the exporter would repeat this on every run.
	if m.Operation == string(validatev1.OperationConverge) || m.Operation == string(validatev1.OperationDestroy) {
		return
	}

	if moduleVersion != "" && moduleVersion != declared {
		dhlog.FromContext(ctx).WarnContext(ctx, configurationFailure(
			"ClusterConfiguration.kubernetesVersion against kubernetesVersion in the \"control-plane-manager\" ModuleConfig",
			fmt.Sprintf("%q and %q. The cluster will be created on %s.", declared, moduleVersion, moduleVersion),
			"the same version in both documents",
			"set both to the same version, or remove ClusterConfiguration.kubernetesVersion").Error())
	}
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
