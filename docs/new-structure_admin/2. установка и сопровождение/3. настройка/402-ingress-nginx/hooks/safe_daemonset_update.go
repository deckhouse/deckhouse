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
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 5 * time.Second,
		ExecutionBurst:       1,
	},
	Queue:       "/modules/ingress-nginx/safe_daemonset_update",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "for_delete",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			ExecuteHookOnSynchronization: pointer.Bool(true),
			ExecuteHookOnEvents:          pointer.Bool(true),
			NamespaceSelector:            internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"lifecycle.apps.kruise.io/state": "PreparingDelete",
				},
			},
			FilterFunc: applyIngressPodFilter,
		},
		{
			Name:                         "proxy_ads",
			ApiVersion:                   "apps.kruise.io/v1alpha1",
			Kind:                         "DaemonSet",
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "proxy-failover",
				},
			},
			FilterFunc: applyDaemonSetFilter,
		},
		{
			Name:                         "failover_ads",
			ApiVersion:                   "apps.kruise.io/v1alpha1",
			Kind:                         "DaemonSet",
			NamespaceSelector:            internal.NsSelector(),
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-failover": "",
					"app":                    "controller",
				},
			},
			FilterFunc: applyDaemonSetFilter,
		},
	},
}, safeControllerUpdate)

func safeControllerUpdate(input *go_hook.HookInput) (err error) {
	controllerPods := input.Snapshots["for_delete"]
	if len(controllerPods) == 0 {
		return nil
	}

	proxys := input.Snapshots["proxy_ads"]
	failovers := input.Snapshots["failover_ads"]

	controllers := set.New()

	proxyMap := make(map[string]daemonSet, len(proxys))
	for _, pc := range proxys {
		ds := pc.(daemonSet)
		proxyMap[ds.ControllerName] = ds
	}

	for _, fc := range failovers {
		ds := fc.(daemonSet)

		proxy, ok := proxyMap[ds.ControllerName]
		if !ok {
			input.LogEntry.Warnf("Proxy DaemonSets not found for %q controller", ds.ControllerName)
			continue
		}

		if proxy.Checksum != ds.Checksum {
			continue
		}

		if proxy.DesiredCount != proxy.UpdatedCount || proxy.DesiredCount != proxy.CurrentReadyCount {
			continue
		}

		if ds.DesiredCount != ds.UpdatedCount || ds.DesiredCount != ds.CurrentReadyCount {
			continue
		}

		controllers.Add(ds.ControllerName)
	}

	for _, sn := range controllerPods {
		podForDelete := sn.(ingressControllerPod)

		if !controllers.Has(podForDelete.ControllerName) {
			input.LogEntry.Warnf("Failover and Proxy DaemonSets not found for %q controller", podForDelete.ControllerName)
			continue
		}

		// postpone main controller's pod update for the first time so that failover controller could catch up with the hook
		if !podForDelete.PostponedUpdate {
			input.LogEntry.Infof("Assuring that %s/%s has met update conditions", podForDelete.ControllerName, podForDelete.Name)
			metadata := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"ingress.deckhouse.io/update-postponed-at": time.Now().Format(time.RFC3339),
					},
				},
			}
			input.PatchCollector.MergePatch(metadata, "v1", "Pod", internal.Namespace, podForDelete.Name)
			continue
		}

		// proxy and failover pods are ready
		metadata := map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"ingress.deckhouse.io/block-deleting": nil,
				},
			},
		}
		input.PatchCollector.MergePatch(metadata, "v1", "Pod", internal.Namespace, podForDelete.Name)
	}

	return nil
}

type ingressControllerPod struct {
	Name            string
	ControllerName  string
	PostponedUpdate bool
}

type daemonSet struct {
	ControllerName    string
	Checksum          string
	DesiredCount      int32
	CurrentReadyCount int32
	UpdatedCount      int32
}

func applyDaemonSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds appsv1.DaemonSet

	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	return daemonSet{
		ControllerName:    strings.TrimSuffix(ds.Labels["name"], "-failover"),
		Checksum:          ds.Annotations["ingress-nginx-controller.deckhouse.io/checksum"],
		DesiredCount:      ds.Status.DesiredNumberScheduled,
		CurrentReadyCount: ds.Status.NumberAvailable,
		UpdatedCount:      ds.Status.UpdatedNumberScheduled,
	}, nil
}

func applyIngressPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod

	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	_, postponedUpdate := pod.Annotations["ingress.deckhouse.io/update-postponed-at"]

	return ingressControllerPod{
		Name:            pod.Name,
		ControllerName:  pod.Labels["name"],
		PostponedUpdate: postponedUpdate,
	}, nil
}
