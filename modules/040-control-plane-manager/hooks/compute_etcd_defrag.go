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
	"gopkg.in/robfig/cron.v2"
)

const (
	etcdDefragEnabledConfigPath    = "controlPlaneManager.etcd.defrag.enabled"
	etcdDefragScheduleConfigPath   = "controlPlaneManager.etcd.defrag.cronSchedule"
	etcdDefragEnabledInternalPath  = "controlPlaneManager.internal.etcdDefrag.enabled"
	etcdDefragScheduleInternalPath = "controlPlaneManager.internal.etcdDefrag.cronSchedule"

	mastersNodeInternalPath        = "controlPlaneManager.internal.mastersNode"
	hasEtcdArbiterNodeInternalPath = "controlPlaneManager.internal.hasEtcdArbiterNode"

	etcdDefragDefaultCronSchedule       = "*/3 * * * *"
	etcdDefragInvalidCronScheduleGroup  = "D8InvalidEtcdDefragCronSchedule"
	etcdDefragInvalidCronScheduleMetric = "d8_invalid_etcd_defrag_cron_schedule"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        moduleQueue,
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
}, handleComputeEtcdDefrag)

func handleComputeEtcdDefrag(_ context.Context, input *go_hook.HookInput) error {
	mastersNode := input.Values.Get(mastersNodeInternalPath).Array()
	hasArbiter := input.Values.Get(hasEtcdArbiterNodeInternalPath).Bool()

	masterCount := len(mastersNode)

	// Default: enable defrag only on clusters with enough etcd members to survive one member being
	// temporarily unavailable during defrag (quorum stays intact).
	defaultEnabled := masterCount >= 3 || (masterCount == 2 && hasArbiter)

	enabled := defaultEnabled
	if input.ConfigValues.Exists(etcdDefragEnabledConfigPath) {
		enabled = input.ConfigValues.Get(etcdDefragEnabledConfigPath).Bool()
	}

	input.MetricsCollector.Expire(etcdDefragInvalidCronScheduleGroup)

	cronSchedule := input.Values.Get(etcdDefragScheduleConfigPath).String()
	input.Logger.Info("etcd defrag schedule from values", "cronSchedule", cronSchedule, "isEmpty", cronSchedule == "")

	if cronSchedule == "" {
		cronSchedule = etcdDefragDefaultCronSchedule
		input.Logger.Info("etcd defrag schedule fallback to constant", "cronSchedule", cronSchedule)
	} else if _, err := cron.Parse("TZ=UTC " + cronSchedule); err != nil {
		input.Logger.Warn("etcd defrag cronSchedule is invalid, falling back to default", "cronSchedule", cronSchedule, "err", err)
		input.MetricsCollector.Set(
			etcdDefragInvalidCronScheduleMetric, 1,
			map[string]string{"cron_schedule": cronSchedule},
			metrics.WithGroup(etcdDefragInvalidCronScheduleGroup),
		)
		cronSchedule = etcdDefragDefaultCronSchedule
	}

	input.Values.Set(etcdDefragEnabledInternalPath, enabled)
	input.Values.Set(etcdDefragScheduleInternalPath, cronSchedule)
	input.Logger.Info("etcd defrag config computed", "enabled", enabled, "cronSchedule", cronSchedule)

	return nil
}
