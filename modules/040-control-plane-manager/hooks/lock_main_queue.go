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
	"context"
	"fmt"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

/*
Description:

	locks deckhouse main queue while control-plane-manager Pods will be rolled out and become ready
	Checks DaemonSets: d8-control-plane-manager and d8-control-plane-manager-etcd-only (if exists)
	Checks Pod readiness for both DaemonSets
*/
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "main",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "cpm_pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "control-plane-manager",
				},
			},
			FilterFunc: lockQueueFilterPod,
		},
		{
			Name:                         "cpm_daemonsets",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
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
			FilterFunc: lockQueueFilterDS,
		},
	},
}, handleLockMainQueue)

type controlPlaneManagerPod struct {
	AppLabel   string
	NodeName   string
	Generation int64
	IsReady    bool
}

type daemonSetInfo struct {
	Name       string
	Generation int64
}

func lockQueueFilterPod(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	podGenerationStr := pod.Labels["pod-template-generation"]
	podGeneration, err := strconv.ParseInt(podGenerationStr, 10, 64)
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

	cpod := controlPlaneManagerPod{
		AppLabel:   pod.Labels["app"],
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

	appLabel := ds.Spec.Selector.MatchLabels["app"]

	return daemonSetInfo{
		Name:       appLabel,
		Generation: ds.GetGeneration(),
	}, nil
}

func handleLockMainQueue(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get("global.clusterIsBootstrapped").Bool() {
		input.Logger.Info("Cluster is not yet bootstrapped, not locking main queue after control-plane-manager update")
		return nil
	}

	dsSnaps, err := sdkobjectpatch.UnmarshalToStruct[daemonSetInfo](input.Snapshots, "cpm_daemonsets")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'cpm_daemonsets' snapshot: %w", err)
	}
	if len(dsSnaps) == 0 {
		return fmt.Errorf("lock the main queue: no control-plane-manager DaemonSet found")
	}

	// Map: app label -> DaemonSet Generation
	dsGenerations := make(map[string]int64)
	for _, ds := range dsSnaps {
		dsGenerations[ds.Name] = ds.Generation
	}

	podsSnaps, err := sdkobjectpatch.UnmarshalToStruct[controlPlaneManagerPod](input.Snapshots, "cpm_pods")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'cpm_pods' snapshot: %w", err)
	}
	if len(podsSnaps) == 0 {
		return fmt.Errorf("lock the main queue: waiting for control-plane-manager Pods being rolled out")
	}

	// Group pods by app label
	podsByApp := make(map[string][]controlPlaneManagerPod)
	for _, pod := range podsSnaps {
		if pod.NodeName == "" {
			continue
		}
		podsByApp[pod.AppLabel] = append(podsByApp[pod.AppLabel], pod)
	}

	// Check each DaemonSet and his pods
	for appLabel, dsGeneration := range dsGenerations {
		pods, exists := podsByApp[appLabel]
		if !exists || len(pods) == 0 {
			return fmt.Errorf("lock the main queue: waiting for %s Pods being rolled out", appLabel)
		}

		expectedReadyPodsCount := 0
		readyCount := 0
		for _, pod := range pods {
			if pod.Generation < dsGeneration {
				return fmt.Errorf("lock the main queue: waiting for %s Pods being rolled out", appLabel)
			}

			expectedReadyPodsCount++

			if pod.IsReady {
				readyCount++
			}
		}

		if readyCount != expectedReadyPodsCount {
			return fmt.Errorf("lock the main queue: waiting for all %s Pods to become Ready", appLabel)
		}
	}

	return nil
}
