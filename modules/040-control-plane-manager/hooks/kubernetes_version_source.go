/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// clusterConfigurationSecretSnapshot is the binding name every hook in this package uses for the
// kube-system/d8-cluster-configuration Secret when it needs the raw, unresolved kubernetesVersion.
const clusterConfigurationSecretSnapshot = "cluster_configuration_secret"

// resolveDeclaredKubernetesVersion returns the operator-declared kubernetesVersion,
// preferring the ModuleConfig control-plane-manager setting over ClusterConfiguration
// when the ModuleConfig value is set and not "Automatic".
func resolveDeclaredKubernetesVersion(input *go_hook.HookInput) string {
	mcVersion := input.Values.Get("controlPlaneManager.kubernetesVersion").String()
	if mcVersion != "" && mcVersion != "Automatic" {
		return mcVersion
	}
	return input.Values.Get("global.clusterConfiguration.kubernetesVersion").String()
}

// rawClusterConfigurationVersion returns the unresolved kubernetesVersion captured from the
// d8-cluster-configuration Secret by sdkvFilterRawClusterConfigurationVersion, or "" when the
// Secret is absent or malformed.
//
// Use this, not global.clusterConfiguration.kubernetesVersion from Values, whenever the literal
// "Automatic" matters: the global discovery hook replaces it with a concrete version before module
// hooks run, so in Values an unpinned cluster is indistinguishable from an explicitly pinned one.
func rawClusterConfigurationVersion(input *go_hook.HookInput) string {
	snaps, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, clusterConfigurationSecretSnapshot)
	if err != nil || len(snaps) == 0 {
		return ""
	}

	return snaps[0]
}

// isPinnedKubernetesVersion reports whether a kubernetesVersion value names a concrete version, as
// opposed to leaving the choice to Deckhouse ("Automatic" or unset).
func isPinnedKubernetesVersion(version string) bool {
	return version != "" && version != "Automatic"
}

// isKubernetesVersionMigrated reports whether the cluster's Kubernetes version no longer depends on
// the deprecated ClusterConfiguration.kubernetesVersion field.
//
// Both arguments must be raw values: mcVersion from the ModuleConfig setting, ccVersion from the
// cluster-configuration.yaml document (see rawClusterConfigurationVersion above for why a resolved
// value would silently break this).
//
// Migrated means removing the ClusterConfiguration field would not change the effective version —
// either the ModuleConfig pins it, so ClusterConfiguration is never consulted, or ClusterConfiguration
// does not pin it either, so resolution lands on the Deckhouse default both before and after.
//
// Two consumers must agree on this predicate: the migration reminder alert
// (alert_migrate_kubernetes_version.go) and the release requirement that will gate the upgrade
// removing the field. Keep it in one place — a divergence would either alert clusters that have
// nothing to migrate, or block their upgrade outright.
func isKubernetesVersionMigrated(mcVersion, ccVersion string) bool {
	return isPinnedKubernetesVersion(mcVersion) || !isPinnedKubernetesVersion(ccVersion)
}
