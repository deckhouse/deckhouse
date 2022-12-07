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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

/*
Description:
	locks deckhouse main queue while prometheus Pod will be Ready
*/
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "main",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "prometheus_main_pod",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-monitoring"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus": "main",
				},
			},
			FilterFunc: lockQueueFilterPod,
		},
	},
}, handleLockMainQueue)

type prometheusPod struct {
	Name    string
	IsReady bool
}

func lockQueueFilterPod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var isReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	cpod := prometheusPod{
		Name:    pod.Name,
		IsReady: isReady,
	}

	return cpod, nil
}

func handleLockMainQueue(input *go_hook.HookInput) error {
	if !input.Values.Get("global.clusterIsBootstrapped").Bool() {
		input.LogEntry.Info("Cluster is not yet bootstrapped, not locking main queue")
		return nil
	}

	if !input.Values.Get("global.discovery.clusterControlPlaneIsHighlyAvailable").Bool() {
		return nil
	}

	snap := input.Snapshots["prometheus_main_pod"]

	if len(snap) == 0 {
		return fmt.Errorf("lock the main queue: waiting for at least one prometheus-main pod with ready status")
	}

	readyCount := 0
	for _, spod := range snap {
		pod := spod.(prometheusPod)

		if pod.IsReady {
			readyCount++
		}
	}

	if readyCount == 0 {
		return fmt.Errorf("lock the main queue: waiting for at least one prometheus-main Pod to become Ready")
	}

	return nil
}
