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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/ingress-nginx/safe_daemonset_update",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controller",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-safe-update": "",
					"app":                       "controller",
				},
			},
			FilterFunc: applyDaemonSetFilter,
		},
		{
			Name:                         "proxy",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-safe-update": "",
					"app":                       "proxy-failover",
				},
			},
			FilterFunc: applyDaemonSetFilter,
		},
		{
			Name:                         "failover",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-ingress-nginx"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"ingress-nginx-failover": "",
					"app":                    "controller",
				},
			},
			FilterFunc: applyDaemonSetFilter,
		},
	},
}, dependency.WithExternalDependencies(safeControllerUpdate))

type IngressFilterResult struct {
	Name     string                 `json:"name"`
	Checksum string                 `json:"checksum"`
	Status   appsv1.DaemonSetStatus `json:"status"`
}

func applyDaemonSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ds := &appsv1.DaemonSet{}

	err := sdk.FromUnstructured(obj, ds)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return IngressFilterResult{
		Name:     ds.Labels["name"],
		Checksum: ds.Annotations["ingress-nginx-controller.deckhouse.io/checksum"],
		Status:   ds.Status,
	}, nil
}

func safeControllerUpdate(input *go_hook.HookInput, dc dependency.Container) (err error) {
	controllers := input.Snapshots["controller"]
	proxys := input.Snapshots["proxy"]
	failovers := input.Snapshots["failover"]

	for _, c := range controllers {
		controller := c.(IngressFilterResult)

		var failoverReady bool
		for _, f := range failovers {
			failover := f.(IngressFilterResult)
			if (controller.Name+"-failover" == failover.Name) && (controller.Checksum == failover.Checksum) {
				if (failover.Status.NumberReady == failover.Status.CurrentNumberScheduled) &&
					(failover.Status.UpdatedNumberScheduled >= failover.Status.DesiredNumberScheduled) {
					failoverReady = true
					break
				}
			}
		}

		if !failoverReady {
			input.LogEntry.Infof("Failover is not yet ready, skipping controller %s", controller.Name)
			continue
		}

		var (
			controllerNeedUpdate bool
			controllerReady      bool
			proxyNeedUpdate      bool
			proxyReady           bool
		)

		if controller.Status.UpdatedNumberScheduled < controller.Status.DesiredNumberScheduled {
			controllerNeedUpdate = true
		}

		if controller.Status.NumberReady == controller.Status.CurrentNumberScheduled {
			controllerReady = true
		}

		for _, p := range proxys {
			proxy := p.(IngressFilterResult)
			if (controller.Name == proxy.Name) && (controller.Checksum == proxy.Checksum) {
				if proxy.Status.NumberReady == proxy.Status.CurrentNumberScheduled {
					proxyReady = true
				}
				if proxy.Status.UpdatedNumberScheduled < proxy.Status.DesiredNumberScheduled {
					proxyNeedUpdate = true
				}
				break
			}
		}

		if proxyReady && controllerReady {
			if controllerNeedUpdate {
				err = daemonSetDeletePodInDs(input, "d8-ingress-nginx", fmt.Sprintf("controller-%s", controller.Name), dc)
				if err != nil {
					return err
				}
			} else if proxyNeedUpdate {
				err = daemonSetDeletePodInDs(input, "d8-ingress-nginx", fmt.Sprintf("proxy-%s-failover", controller.Name), dc)
				if err != nil {
					return err
				}
			}
		}

		err = daemonSetDeleteCrashLoopBackPods(input, "d8-ingress-nginx", fmt.Sprintf("controller-%s", controller.Name), dc)
		if err != nil {
			return err
		}

		err = daemonSetDeleteCrashLoopBackPods(input, "d8-ingress-nginx", fmt.Sprintf("proxy-%s-failover", controller.Name), dc)
		if err != nil {
			return err
		}
	}

	return nil
}

func daemonSetDeletePodInDs(input *go_hook.HookInput, namespace, dsName string, dc dependency.Container) error {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	podList, err := getDaemonSetPodList(k8, dsName)
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return nil
	}

	podNameToKill := podList.Items[0].Name
	podGenerationToKill := podList.Items[0].Labels["pod-template-generation"]

	input.LogEntry.Infof("Deleting controller pod %s of generation %s", podNameToKill, podGenerationToKill)

	err = k8.CoreV1().Pods(namespace).Delete(context.TODO(), podNameToKill, metav1.DeleteOptions{})
	if err != nil {
		input.LogEntry.Error(err)
	}

	return nil
}

func daemonSetDeleteCrashLoopBackPods(input *go_hook.HookInput, namespace, dsName string, dc dependency.Container) error {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	podList, err := getDaemonSetPodList(k8, dsName)
	if err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return nil
	}

	var podsToKill []string
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if (containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff") ||
				(containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "Error") {
				podsToKill = append(podsToKill, pod.Name)
			}
		}
	}

	for _, podName := range podsToKill {
		err = k8.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metav1.DeleteOptions{})
		if err != nil {
			input.LogEntry.Error(err)
		}
	}

	return nil
}

func getDaemonSetPodList(client k8s.Client, dsName string) (*v1.PodList, error) {
	daemonset, err := client.AppsV1().DaemonSets(namespace).Get(context.TODO(), dsName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	generation := daemonset.Generation

	selector, err := metav1.LabelSelectorAsSelector(daemonset.Spec.Selector)
	if err != nil {
		return nil, err
	}
	podTemplateGenerationReq, err := labels.NewRequirement("pod-template-generation", selection.NotEquals, []string{strconv.FormatInt(generation, 10)})
	if err != nil {
		return nil, err
	}
	selector = selector.Add(*podTemplateGenerationReq)

	return client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
}
