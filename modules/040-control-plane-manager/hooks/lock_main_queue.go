/*
Copyright 2021 Flant JSC

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
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

/*
Description:

	locks deckhouse main queue while control-plane-manager Pod will be rolled out and become ready
	Checks Daemonset: d8-control-plane-manager exists
	Checks Pod readiness
*/
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "main",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "cpm_pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "d8-control-plane-manager",
				},
			},
			FilterFunc: lockQueueFilterPod,
		},

		{
			Name:                         "cpm_ds",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnEvents:          pointer.Bool(false),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-control-plane-manager"},
			},
			FilterFunc: lockQueueFilterDS,
		},
	},
}, handleLockMainQueue)

type controlPlaneManagerPod struct {
	NodeName   string
	Generation string
	IsReady    bool
}

func lockQueueFilterPod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	podGeneration := pod.Labels["pod-template-generation"]

	var isReady bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}

	cpod := controlPlaneManagerPod{
		Generation: podGeneration,
		NodeName:   pod.Spec.NodeName,
		IsReady:    isReady,
	}

	return cpod, nil
}
func lockQueueFilterDS(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds appsv1.DaemonSet

	err := sdk.FromUnstructured(unstructured, &ds)
	if err != nil {
		return nil, err
	}

	return ds.GetGeneration(), nil
}

func handleLockMainQueue(input *go_hook.HookInput) error {
	if !input.Values.Get("global.clusterIsBootstrapped").Bool() {
		input.LogEntry.Info("Cluster is not yet bootstrapped, not locking main queue after control-plane-manager update")
		return nil
	}

	// Lock deckhouse main queue while the control-plane is updating.
	snap := input.Snapshots["cpm_ds"]
	if len(snap) == 0 || snap[0] == nil {
		return fmt.Errorf("lock the main queue: no control-plane-manager DaemonSet found")
	}

	dsGeneration := snap[0].(int64)
	dsGenerationStr := strconv.FormatInt(dsGeneration, 10)

	snap = input.Snapshots["cpm_pods"]

	if len(snap) == 0 {
		return fmt.Errorf("lock the main queue: waiting for control-plane-manager Pods being rolled out")
	}

	expectedReadyPodsCount := 0
	readyCount := 0
	for _, spod := range snap {
		pod := spod.(controlPlaneManagerPod)
		if pod.NodeName == "" || pod.Generation != dsGenerationStr {
			continue
		}
		expectedReadyPodsCount++

		if pod.IsReady {
			readyCount++
		}
	}

	if readyCount != expectedReadyPodsCount {
		return fmt.Errorf("lock the main queue: waiting for all control-plane-manager Pods to become Ready")
	}

	return nil
}
