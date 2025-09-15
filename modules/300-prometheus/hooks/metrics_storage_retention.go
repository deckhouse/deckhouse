/*
Copyright 2022 Flant JSC

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

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/prometheus/metrics_storage_retention",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
}, storageRetentionMetricHandler)

func storageRetentionMetricHandler(_ context.Context, input *go_hook.HookInput) error {
	retentionDaysMain := input.Values.Get("prometheus.retentionDays")
	retentionDaysLongterm := input.Values.Get("prometheus.longtermRetentionDays")

	input.MetricsCollector.Expire("prometheus_disk_hook")

	input.MetricsCollector.Set(
		"d8_prometheus_storage_retention_days",
		retentionDaysMain.Float(),
		map[string]string{
			"prometheus": "main",
		},
		metrics.WithGroup("prometheus_disk_hook"),
	)

	input.MetricsCollector.Set(
		"d8_prometheus_storage_retention_days",
		retentionDaysLongterm.Float(),
		map[string]string{
			"prometheus": "longterm",
		},
		metrics.WithGroup("prometheus_disk_hook"),
	)

	return nil
}
