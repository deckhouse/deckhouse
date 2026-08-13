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
)

/*
Description:
	Alerts when the cluster Kubernetes version is still owned by the deprecated
	ClusterConfiguration.kubernetesVersion instead of ModuleConfig control-plane-manager.
*/

const (
	unsetKubernetesVersionMetricGroup = "D8UnsetKubernetesVersionInModuleConfig"
	unsetKubernetesVersionMetricName  = "d8_unset_kubernetes_version_in_module_config"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/control-plane-manager/alerting",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, checkKubernetesVersionMigration)

func checkKubernetesVersionMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(unsetKubernetesVersionMetricGroup)

	mcVersion := input.Values.Get("controlPlaneManager.kubernetesVersion").String()
	trackingDefault := input.Values.Get("global.discovery.kubernetesVersionIsDefault").Bool()

	if kubernetesVersionNeedsMigration(mcVersion, trackingDefault) {
		input.MetricsCollector.Set(
			unsetKubernetesVersionMetricName, 1,
			map[string]string{},
			metrics.WithGroup(unsetKubernetesVersionMetricGroup),
		)
	}

	return nil
}

// kubernetesVersionNeedsMigration reports whether the deprecated ClusterConfiguration field still
// decides the cluster version.
//
// Any ModuleConfig setting clears the alert, Default included: presence of that field, not its
// value, is what takes ownership away from ClusterConfiguration.
//
// A ClusterConfiguration that pins nothing — absent, Automatic, Default — is not something to
// migrate: the resolved version is the release default either way, exactly as it would be with
// ModuleConfig Default. Alerting there would fire on every fresh cluster with nothing to fix.
//
// Hence trackingDefault, from global.discovery.kubernetesVersionIsDefault, rather than the
// ClusterConfiguration value itself: global.clusterConfiguration.kubernetesVersion cannot answer
// this question, because the hook publishing it substitutes Automatic for the concrete release
// default before writing it (global-hooks/discovery/cluster_configuration.go), which makes an
// Automatic cluster indistinguishable from a pinned one. The flag is derived from a separate
// snapshot of the raw Secret and is true exactly when nothing pins the version anywhere.
func kubernetesVersionNeedsMigration(mcVersion string, trackingDefault bool) bool {
	return mcVersion == "" && !trackingDefault
}
