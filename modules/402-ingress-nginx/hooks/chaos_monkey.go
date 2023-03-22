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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

type ingressDaemonSetFilterResult struct {
	ControllerName  string
	LabelSelector   map[string]string
	DesiredReplicas int32
	ReadyReplicas   int32
}

type ingressControllerChaosConfig struct {
	ControllerName     string
	ChaosMonkeyEnabled bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/10 * * * *"},
	},
	Queue: "/modules/ingress-nginx/chaos_monkey",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controllers",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "IngressNginxController",
			FilterFunc:                   chaosMonkeyApplyControllerFilter,
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
		},
		{
			Name:              "daemonsets",
			ApiVersion:        "apps/v1",
			Kind:              "DaemonSet",
			NamespaceSelector: internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc:                   applyIngressDaemonSetFilter,
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
		},
	},
}, dependency.WithExternalDependencies(chaosMonkey))

func chaosMonkeyApplyControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ingressControllerName, _, err := unstructured.NestedString(obj.UnstructuredContent(), "metadata", "name")
	if err != nil {
		return nil, fmt.Errorf(`failed to get "metadata.name" field from object %+v: %s`, *obj, err)
	}

	chaosEnabled, _, err := unstructured.NestedBool(obj.UnstructuredContent(), "spec", "chaosMonkey")
	if err != nil {
		return nil, fmt.Errorf(`failed to get "spec.chaosEnabled" field from object %+v: %s`, *obj, err)
	}

	return ingressControllerChaosConfig{
		ControllerName:     ingressControllerName,
		ChaosMonkeyEnabled: chaosEnabled,
	}, nil
}

func applyIngressDaemonSetFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	d := &appsv1.DaemonSet{}

	err := sdk.FromUnstructured(obj, d)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return ingressDaemonSetFilterResult{
		ControllerName:  d.Labels["name"],
		LabelSelector:   d.Spec.Selector.MatchLabels,
		DesiredReplicas: d.Status.DesiredNumberScheduled,
		ReadyReplicas:   d.Status.NumberReady,
	}, nil
}

func chaosMonkey(input *go_hook.HookInput, dc dependency.Container) (err error) {
	controllers := input.Snapshots["controllers"]
	daemonsets := input.Snapshots["daemonsets"]

	chaosMonkeyEnabled := make(map[string]bool)
	for _, c := range controllers {
		controller := c.(ingressControllerChaosConfig)
		chaosMonkeyEnabled[controller.ControllerName] = controller.ChaosMonkeyEnabled
	}

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	for _, ds := range daemonsets {
		res := ds.(ingressDaemonSetFilterResult)
		if !chaosMonkeyEnabled[res.ControllerName] {
			input.LogEntry.Debugf("chaos monkey is disabled for controller %q, skipping", res.ControllerName)
			continue
		}

		if res.DesiredReplicas != res.ReadyReplicas {
			input.LogEntry.Debugf("controller %q replicase aren't ready %d/%d, skipping", res.ControllerName, res.ReadyReplicas, res.DesiredReplicas)
			continue
		}

		podList, err := kubeClient.CoreV1().
			Pods(internal.Namespace).
			List(context.TODO(), metav1.ListOptions{LabelSelector: labels.FormatLabels(res.LabelSelector)})
		if err != nil {
			return err
		}

		if len(podList.Items) < 2 {
			input.LogEntry.Debugf("at least two pods for controller %q are required, skipping", res.ControllerName)
			return nil
		}

		oldestPod := podList.Items[0]
		for _, pod := range podList.Items {
			if pod.CreationTimestamp.Before(&oldestPod.CreationTimestamp) {
				oldestPod = pod
			}
		}

		err = kubeClient.CoreV1().
			Pods(internal.Namespace).
			EvictV1(context.TODO(), &policyv1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: oldestPod.Name}})
		if err != nil {
			input.LogEntry.Infof("can't evict ingress controller pod %q: %v", oldestPod.Name, err)
		}
	}

	return nil
}
