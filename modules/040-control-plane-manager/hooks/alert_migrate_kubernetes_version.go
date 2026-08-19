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
	Alerts while the deprecated ClusterConfiguration.kubernetesVersion field is still present,
	asking to move it into ModuleConfig control-plane-manager.
*/

const (
	obsoleteKubernetesVersionFieldMetricGroup = "D8ObsoleteKubernetesVersionFieldInClusterConfiguration"
	obsoleteKubernetesVersionFieldMetricName  = "d8_obsolete_kubernetes_version_field_in_cluster_configuration"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/control-plane-manager/alerting",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, checkKubernetesVersionMigration)

func checkKubernetesVersionMigration(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteKubernetesVersionFieldMetricGroup)
	ccVersion := input.Values.Get("global.clusterConfiguration.kubernetesVersion").String()

	if ccVersion != "" {
		input.MetricsCollector.Set(
			obsoleteKubernetesVersionFieldMetricName, 1,
			map[string]string{},
			metrics.WithGroup(obsoleteKubernetesVersionFieldMetricGroup),
		)
	}

	return nil
}
