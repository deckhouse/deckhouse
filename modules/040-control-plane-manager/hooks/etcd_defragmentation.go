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
	"errors"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:     moduleQueue + "/automatic_defragmentation",
	OnStartup: &go_hook.OrderedConfig{Order: 5},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "every_hour",
			Crontab: "27 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		etcdMaintenanceConfig,
		etcdSecretK8sConfig,
	},
}, dependency.WithExternalDependencies(handleTriggerETCDAutomaticDefragmentation))

func handleTriggerETCDAutomaticDefragmentation(input *go_hook.HookInput, dc dependency.Container) error {
	etcdClient, err := getETCDClientFromSnapshots(input, dc)
	if err != nil {
		if errors.Is(err, ErrEmptyEtcdSnapshot) {
			return nil
		}

		return err
	}
	defer etcdClient.Close()

	for _, endpointRaw := range input.Snapshots[etcdEndpointsSnapshotName] {
		instance := endpointRaw.(*etcdInstance)
		status, err := etcdClient.Status(context.TODO(), instance.Endpoint)
		if err != nil {
			input.LogEntry.Errorf("cannot get current db usage from %s: %v", err, instance.PodName)
			continue
		}

		if float64(status.DbSize)/float64(instance.MaxDbSize) < 0.9 {
			input.LogEntry.Debugf("Etcd instanse '%s' does not need to defrag", instance.PodName)
			continue
		}

		input.LogEntry.Warnf("Start defrag etcd instanse '%s' %d/%d", instance.PodName, status.DbSize, instance.MaxDbSize)
		_, err = etcdClient.Defragment(context.TODO(), instance.Endpoint)
		if err != nil {
			input.MetricsCollector.Inc("etcd_defragmentation_failed_total", map[string]string{
				"pod_name": instance.PodName,
				"node":     instance.Node,
			})
			input.LogEntry.Errorf("Defrag etcd '%s' instanse finished with err: %v", instance.PodName, err)
			continue
		}

		input.MetricsCollector.Inc("etcd_defragmentation_success_total", map[string]string{
			"pod_name": instance.PodName,
			"node":     instance.Node,
		})

		input.LogEntry.Infof("Defrag etcd '%s' instanse finished successfully", instance.PodName)
	}

	return nil
}
