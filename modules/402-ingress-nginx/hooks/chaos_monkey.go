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
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

type DeploymentFilterResult struct {
	ControllerName string
	LabelSelector  map[string]string
}

type IngressControllerChaosConfig struct {
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
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
		},
		{
			Name:       "deployments",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{namespace},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "controller",
				},
			},
			FilterFunc:                   applyDeploymentFilter,
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
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

	return IngressControllerChaosConfig{ControllerName: ingressControllerName, ChaosMonkeyEnabled: chaosEnabled}, nil
}

func applyDeploymentFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	d := &appsv1.Deployment{}

	err := sdk.FromUnstructured(obj, d)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes object: %v", err)
	}

	return DeploymentFilterResult{
		ControllerName: d.Labels["name"],
		LabelSelector:  d.Spec.Selector.MatchLabels,
	}, nil
}

func chaosMonkey(input *go_hook.HookInput, dc dependency.Container) (err error) {
	controllers := input.Snapshots["controllers"]
	deployments := input.Snapshots["deployments"]

	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	for _, c := range controllers {
		controller := c.(IngressControllerChaosConfig)
		if !controller.ChaosMonkeyEnabled {
			continue
		}

		selector, err := getPodSelector(controller.ControllerName, deployments)
		if err != nil {
			continue
		}

		podList, err := kubeClient.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labels.FormatLabels(selector)})
		if err != nil {
			return err
		}

		if len(podList.Items) == 0 {
			return nil
		}

		podToEvict := getPodWithMostNeighbors(podList)
		if podToEvict == nil {
			return nil
		}

		err = kubeClient.CoreV1().Pods(namespace).Evict(context.TODO(), &v1beta1.Eviction{ObjectMeta: metav1.ObjectMeta{Name: podToEvict.Name}})
		if err != nil {
			input.LogEntry.Infof("can't Evict Ingress Controller's Pod %s/%s due to PDB constraints: %s", podToEvict.Namespace, podToEvict.Name, err)
		}
	}

	return nil
}

func getPodWithMostNeighbors(podList *v1.PodList) *v1.Pod {
	var (
		nodeNameToPodMapping = make(map[string]int)
		podsWithNode         = make([]v1.Pod, 0, len(podList.Items))
	)

	for _, pod := range podList.Items {
		if nodeName := pod.Spec.NodeName; len(nodeName) != 0 {
			nodeNameToPodMapping[nodeName]++
			podsWithNode = append(podsWithNode, pod)
		}
	}

	sort.Slice(podsWithNode, func(i, j int) bool {
		return podsWithNode[i].CreationTimestamp.Before(&podsWithNode[j].CreationTimestamp)
	})

	for _, pod := range podsWithNode {
		if nodeNameToPodMapping[pod.Spec.NodeName] > 1 {
			return &pod
		}
	}

	return nil
}

func getPodSelector(controllerName string, deployments []go_hook.FilterResult) (map[string]string, error) {
	for _, d := range deployments {
		deployment := d.(DeploymentFilterResult)
		if deployment.ControllerName == controllerName {
			return deployment.LabelSelector, nil
		}
	}

	return nil, fmt.Errorf("deployment for ingress controller %v not found", controllerName)
}
