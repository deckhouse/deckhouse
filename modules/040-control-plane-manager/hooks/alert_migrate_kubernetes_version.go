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
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
)

/*
TODO(kubernetesVersion-deprecation): T+2 remove — this entire file is temporary. It exists to nag about a
migration, so it is deleted in the same release that removes kubernetesVersion from
ClusterConfiguration, together with the D8ObsoleteKubernetesVersionInClusterConfiguration alert in
monitoring/prometheus-rules/mc-migration.yaml and its entry in
docs/documentation/_data/deckhouse-alerts.yml.

Do not delete it earlier than that release: until the field is gone the alert is the only thing
telling operators that their cluster still depends on it, and the value it publishes is what the
release gate reads.

Description:
	Reminds the operator to move an explicit kubernetesVersion from the deprecated
	ClusterConfiguration field to the control-plane-manager ModuleConfig setting.

	The Secret binding is not optional: deciding whether ClusterConfiguration pins a version at all
	requires its raw value. global.clusterConfiguration.kubernetesVersion cannot answer that — the
	global discovery hook substitutes a concrete version for "Automatic" before module hooks run,
	which would make this alert fire on every cluster, including those with nothing to migrate.
*/

// TODO(kubernetesVersion-deprecation): T+1 measure — this metric is the adoption counter for the
// migration: it is set exactly on clusters that still depend on the deprecated field. Read it
// across the fleet before arming the release gate, because that gate blocks upgrades for every
// cluster still counted here.
const (
	obsoleteKubernetesVersionMetricGroup = "D8ObsoleteKubernetesVersionInClusterConfiguration"
	obsoleteKubernetesVersionMetricName  = "d8_obsolete_kubernetes_version_in_cluster_configuration"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/control-plane-manager/alerting",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       clusterConfigurationSecretSnapshot,
			ApiVersion: "v1",
			Kind:       "Secret",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-configuration"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: sdkvFilterRawClusterConfigurationVersion,
		},
	},
}, checkKubernetesVersionMigration)

func checkKubernetesVersionMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteKubernetesVersionMetricGroup)

	mcVersion := input.Values.Get("controlPlaneManager.kubernetesVersion").String()
	ccVersion := rawClusterConfigurationVersion(input)
	migrated := isKubernetesVersionMigrated(mcVersion, ccVersion)

	// Publish for the T+2 DeckhouseRelease gate (requirements/check.go). Same predicate as the
	// alert below — a divergence would either alert clusters that have nothing to migrate, or
	// block their upgrade outright.
	requirements.SaveValue(KubernetesVersionMigratedRequirementKey, migrated)

	if !migrated {
		input.MetricsCollector.Set(
			obsoleteKubernetesVersionMetricName, 1,
			map[string]string{},
			metrics.WithGroup(obsoleteKubernetesVersionMetricGroup),
		)
	}

	return nil
}
