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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type etc

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
		getEtcdEndpointConfig(func(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var pod corev1.Pod

			err := sdk.FromUnstructured(unstructured, &pod)
			if err != nil {
				return nil, err
			}

			var ip string
			if pod.Spec.HostNetwork {
				ip = pod.Status.HostIP
			} else {
				ip = pod.Status.PodIP
			}

			return etcdEndpointString(ip), nil
		}),
		etcdSecretK8sConfig,
	},
}, dependency.WithExternalDependencies(handleTriggerETCDAutomaticDefragmentation))

func handleTriggerETCDAutomaticDefragmentation(input *go_hook.HookInput, dc dependency.Container) error {
	if len(input.Snapshots["last_defrag_ds"]) == 0 {
		return nil
	}

	defragDisabled := input.Values.Get("controlPlaneManager.disableAutoDefragmentation").Bool()
	if defragDisabled {
		return nil
	}

	etcdClient, err := getETCDClientFromSnapshots(input, dc)
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	var lastErr error
	var dbSize int64
	for _, endpointRaw := range input.Snapshots["etcd_endpoints"] {
		lastErr = nil
		status, err := etcdClient.Status(context.TODO(), endpointRaw.(string))
		if err != nil {
			lastErr = err
			continue
		}

		dbSize = status.DbSize
		break
	}

	if lastErr != nil {
		input.LogEntry.Errorf("Cannot get db size: %v", lastErr)
		return nil
	}

	lastDefragTime := input.Snapshots["last_defrag_ds"][0].(string)
	input.Values.Set("controlPlaneManager.internal.lastDefragTime", lastDefragTime)

	return nil
}
