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
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// clusterConfigurationSecretSnapshot is the binding name every hook in this package uses for the
// kube-system/d8-cluster-configuration Secret when it needs the raw, unresolved kubernetesVersion.
const clusterConfigurationSecretSnapshot = "cluster_configuration_secret"

// automaticKubernetesVersion is the sentinel meaning "let Deckhouse pick the version".
const automaticKubernetesVersion = "Automatic"

// KubernetesVersionMigratedRequirementKey is the requirements.SaveValue key that records whether
// kubernetesVersion no longer depends on the deprecated ClusterConfiguration field.
//
// Published by the migration alert hook starting at T+0 so that a DeckhouseRelease at T+2 can
// gate on RegisterCheck("kubernetesVersionMigrated") — the installed Deckhouse evaluates the
// candidate release, so the value must already be present before the requirement appears.
const KubernetesVersionMigratedRequirementKey = "controlPlaneManager:kubernetesVersionMigrated"

// rawClusterConfiguration captures only the raw (possibly literal "Automatic") kubernetesVersion
// field from the embedded ClusterConfiguration YAML — unlike
// global.clusterConfiguration.kubernetesVersion in Values, which the global discovery hook has
// already resolved to a concrete version by the time module hooks run.
type rawClusterConfiguration struct {
	KubernetesVersion string `json:"kubernetesVersion"`
}

// sdkvFilterRawClusterConfigurationVersion returns the unresolved kubernetesVersion from the
// d8-cluster-configuration Secret. Shared by the migration alert (and any future requirement that
// needs the literal "Automatic").
func sdkvFilterRawClusterConfigurationVersion(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret
	if err := sdk.FromUnstructured(unstructured, &secret); err != nil {
		return nil, err
	}

	var cc rawClusterConfiguration
	if err := yaml.Unmarshal(secret.Data["cluster-configuration.yaml"], &cc); err != nil {
		// Malformed/absent ClusterConfiguration is unrelated — ignore.
		return "", nil
	}

	return cc.KubernetesVersion, nil
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
	return version != "" && version != automaticKubernetesVersion
}

// isKubernetesVersionMigrated reports whether the cluster's Kubernetes version no longer depends on
// the deprecated ClusterConfiguration.kubernetesVersion field.
//
// Both arguments must be raw values: mcVersion from the ModuleConfig setting, ccVersion from the
// cluster-configuration.yaml document (see rawClusterConfigurationVersion above for why a resolved
// value would silently break this).
//
// Migrated means removing the ClusterConfiguration field would not change the effective version —
// either the ModuleConfig setting is present, so ClusterConfiguration is never consulted, or
// ClusterConfiguration does not pin a version either, so resolution lands on the Deckhouse default
// both before and after.
//
// Presence, not pinning, is what makes the ModuleConfig authoritative: an explicit "Automatic"
// there already means "track the Deckhouse default, ignore ClusterConfiguration" (see
// resolveTargetKubernetesVersion in global-hooks/discovery/cluster_configuration.go), so such a
// cluster has nothing left to migrate and must not be nagged by the alert or blocked at T+2.
//
// Two consumers must agree on this predicate: the migration reminder alert
// (alert_migrate_kubernetes_version.go), which also publishes it via
// requirements.SaveValue(KubernetesVersionMigratedRequirementKey), and the release
// requirement that will gate the upgrade removing the field
// (requirements.RegisterCheck("kubernetesVersionMigrated")). Keep it in one place —
// a divergence would either alert clusters that have nothing to migrate, or block
// their upgrade outright.
func isKubernetesVersionMigrated(mcVersion, ccVersion string) bool {
	return mcVersion != "" || !isPinnedKubernetesVersion(ccVersion)
}
