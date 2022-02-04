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
	"math"
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	autoscaler "github.com/deckhouse/deckhouse/modules/302-vertical-pod-autoscaler/hooks/internal/vertical-pod-autoscaler/v1"
)

/*
Overview:
   1. All system components require resource requests, managed by vpa.
   2. Sum of all resource requests should not exceed manually configured resources limits.
   3. We expect that resources limits to be allocated fairly between vpa requests.
We have 3 groups of resources:
   1. ControlPlane - resources for pods in control-plane (kube-controller-manager, kube-scheduler, kube-apiserver, etcd).
   2. Master - vpa resources, working on master nodes (label "workload-resource-policy.deckhouse.io: master").
   3. EveryNode - vpa resources, working on every node (label "workload-resource-policy.deckhouse.io: every-node").
Calculate steps:
   1. We calculate sum of uncappedTargets requests for all vpa resources in Master group, and proportionally sets MaxAllowed values for this resources,
      based on resources requests from global config for Master group. If newly calculated value differs less than 10% from old value, new value set is skipped.
   2. We calculate sum of uncappedTargets requests for all vpa resources in EveryNode group, and proportionally sets MaxAllowed values for this resources,
      based on resources requests from global config for EveryNode group. If newly calculated value differs less than 10% from old value, new value set is skipped.
Hook start conditions:
   1. If uncappedTarget value changed in vpa with labels "workload-resource-policy.deckhouse.io: master" or "workload-resource-policy.deckhouse.io: every-node".
   2. If user changed global.modules.resourcesRequests values.
   3. By crontab to process situation, if nodes resources changed.
*/

const (
	groupLabelKey    = "workload-resource-policy.deckhouse.io"
	everyNodeLabel   = "every-node"
	masterLabel      = "master"
	vpaAPIVersion    = "autoscaling.k8s.io/v1"
	thresholdPercent = 10
)

type VPA struct {
	Name                     string
	Namespace                string
	Label                    string
	ContainerRecommendations []autoscaler.RecommendedContainerResources
}

func applyDeckhousePodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var isReady bool

	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			isReady = true
			break
		}
	}
	return isReady, nil
}

func applyVpaResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	v := &autoscaler.VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(obj, v)
	if err != nil {
		return nil, fmt.Errorf("cannot parse vpa object from unstructured: %v", err)
	}

	if v.Status.Recommendation == nil {
		return nil, nil
	}
	recommendations := v.Status.Recommendation.ContainerRecommendations

	c := &VPA{}
	c.Name = v.Name
	c.Namespace = v.Namespace
	c.Label = v.Labels[groupLabelKey]
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].ContainerName < recommendations[j].ContainerName
	})
	c.ContainerRecommendations = recommendations

	return c, nil
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
		Queue:        "/modules/vertical-pod-autoscaler",
		Schedule: []go_hook.ScheduleConfig{
			{Name: "vpaCron", Crontab: "*/30 * * * *"},
		},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:                   "Vpa",
				WaitForSynchronization: pointer.BoolPtr(false),
				ExecuteHookOnEvents:    pointer.BoolPtr(false),
				ApiVersion:             vpaAPIVersion,
				Kind:                   "VerticalPodAutoscaler",
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "heritage",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"deckhouse"},
						},
						{
							Key:      groupLabelKey,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{everyNodeLabel, masterLabel},
						},
					},
				},
				FilterFunc: applyVpaResourcesFilter,
			},
			{
				Name:                   "DeckhousePod",
				WaitForSynchronization: pointer.BoolPtr(false),
				ExecuteHookOnEvents:    pointer.BoolPtr(false),
				ApiVersion:             "v1",
				Kind:                   "Pod",
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"deckhouse"},
						},
					},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-system"},
					},
				},
				FilterFunc: applyDeckhousePodFilter,
			},
		},
	}, updateVpaResources)
)

func updateVpaResources(input *go_hook.HookInput) error {
	var (
		configEveryNodeMilliCPU  float64
		configEveryNodeMemory    float64
		configMasterNodeMilliCPU float64
		configMasterNodeMemory   float64

		totalRequestsMasterNodeMilliCPU float64
		totalRequestsMasterNodeMemory   float64
		totalRequestsEveryNodeMilliCPU  float64
		totalRequestsEveryNodeMemory    float64

		deckhousePodIsReady bool
	)

	// Check if Deckhouse pod is ready
	podSnapshots := input.Snapshots["DeckhousePod"]
	if len(podSnapshots) == 0 {
		input.LogEntry.Info("deckhouse pod is not ready, skipping")
		return nil
	}

	for _, podSnapshot := range podSnapshots {
		if podSnapshot == nil {
			continue
		}
		deckhousePodIsReady = podSnapshot.(bool)
		if deckhousePodIsReady {
			break
		}
	}

	if !deckhousePodIsReady {
		input.LogEntry.Info("deckhouse pod is not ready, skipping")
		return nil
	}

	configEveryNodeMilliCPU, err := getPathFloat64(input, "global.internal.modules.resourcesRequests.milliCpuEveryNode")
	if err != nil {
		return err
	}
	configEveryNodeMemory, err = getPathFloat64(input, "global.internal.modules.resourcesRequests.memoryEveryNode")
	if err != nil {
		return err
	}
	configMasterNodeMilliCPU, err = getPathFloat64(input, "global.internal.modules.resourcesRequests.milliCpuMaster")
	if err != nil {
		return err
	}
	configMasterNodeMemory, err = getPathFloat64(input, "global.internal.modules.resourcesRequests.memoryMaster")
	if err != nil {
		return err
	}

	snapshots := input.Snapshots["Vpa"]
	if len(snapshots) == 0 {
		return nil
	}

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}

		v := snapshot.(*VPA)

		for _, r := range v.ContainerRecommendations {
			ut := r.UncappedTarget

			switch v.Label {
			case masterLabel:
				totalRequestsMasterNodeMilliCPU += float64(ut.Cpu().MilliValue())
				totalRequestsMasterNodeMemory += float64(ut.Memory().Value())
			case everyNodeLabel:
				totalRequestsEveryNodeMilliCPU += float64(ut.Cpu().MilliValue())
				totalRequestsEveryNodeMemory += float64(ut.Memory().Value())
			}
		}
	}

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		v := snapshot.(*VPA)

		input.PatchCollector.Filter(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var (
				recommendationsMilliCPU float64
				recommendationsMemory   float64
				containerPolicies       []autoscaler.ContainerResourcePolicy
			)

			vpa := &autoscaler.VerticalPodAutoscaler{}
			err := sdk.FromUnstructured(obj, vpa)
			if err != nil {
				return nil, fmt.Errorf("cannot parse vpa object from unstructured: %v", err)
			}

			// calculate cpu and memory maxAllowed values for containers
			for _, container := range vpa.Status.Recommendation.ContainerRecommendations {
				switch vpa.Labels[groupLabelKey] {
				case masterLabel:
					recommendationsMilliCPU = float64(container.UncappedTarget.Cpu().MilliValue()) * (configMasterNodeMilliCPU / totalRequestsMasterNodeMilliCPU)
					recommendationsMemory = float64(container.UncappedTarget.Memory().Value()) * (configMasterNodeMemory / totalRequestsMasterNodeMemory)
				case everyNodeLabel:
					recommendationsMilliCPU = float64(container.UncappedTarget.Cpu().MilliValue()) * (configEveryNodeMilliCPU / totalRequestsEveryNodeMilliCPU)
					recommendationsMemory = float64(container.UncappedTarget.Memory().Value()) * (configEveryNodeMemory / totalRequestsEveryNodeMemory)
				}

				if isInfinity(recommendationsMilliCPU) {
					return nil, fmt.Errorf("recommendationsMilliCPU is infinity number")
				}

				if isInfinity(recommendationsMemory) {
					return nil, fmt.Errorf("recommendationsMemory is infinity number")
				}

				newContainerPolicy := autoscaler.ContainerResourcePolicy{
					ContainerName: container.ContainerName,
					MaxAllowed: v1.ResourceList{
						v1.ResourceCPU:    *resource.NewMilliQuantity(int64(recommendationsMilliCPU), resource.BinarySI),
						v1.ResourceMemory: *resource.NewQuantity(int64(recommendationsMemory), resource.DecimalExponent),
					},
				}
				containerPolicies = append(containerPolicies, newContainerPolicy)
			}

			if vpa.Spec.ResourcePolicy != nil {
				// if percent threshold between newly calculated and old values < percentThreshold, use old values
				containerPoliciesThreshold(containerPolicies, vpa.Spec.ResourcePolicy.ContainerPolicies)
			}

			vpa.Spec.ResourcePolicy = &autoscaler.PodResourcePolicy{ContainerPolicies: containerPolicies}

			resObj, err := sdk.ToUnstructured(vpa)
			if err != nil {
				return nil, fmt.Errorf("cannot parse unstructured to object: %v", err)
			}
			return resObj, nil
		}, vpaAPIVersion, "VerticalPodAutoscaler", v.Namespace, v.Name)
	}
	return nil
}

func getPathFloat64(input *go_hook.HookInput, path string) (float64, error) {
	if !input.Values.Exists(path) {
		return 0, fmt.Errorf("%s must be set", path)
	}
	return input.Values.Get(path).Float(), nil
}

func containerPoliciesThreshold(newPolicies []autoscaler.ContainerResourcePolicy, oldPolicies []autoscaler.ContainerResourcePolicy) {
	for i := range newPolicies {
		cpu := newPolicies[i].MaxAllowed.Cpu().MilliValue()
		memory := newPolicies[i].MaxAllowed.Memory().Value()
		for j := range oldPolicies {
			if oldPolicies[j].ContainerName != newPolicies[i].ContainerName {
				continue
			}
			cpuPercent := calculatePercent(cpu, oldPolicies[j].MaxAllowed.Cpu().MilliValue())
			memoryPercent := calculatePercent(memory, oldPolicies[j].MaxAllowed.Memory().Value())
			if inThreshold(cpuPercent) {
				cpu = oldPolicies[j].MaxAllowed.Cpu().MilliValue()
			}
			if inThreshold(memoryPercent) {
				memory = oldPolicies[j].MaxAllowed.Memory().Value()
			}
			newPolicies[i].MaxAllowed = v1.ResourceList{
				v1.ResourceCPU:    *resource.NewMilliQuantity(cpu, resource.BinarySI),
				v1.ResourceMemory: *resource.NewQuantity(memory, resource.DecimalExponent),
			}
		}
	}
}

func isInfinity(value float64) bool {
	return math.IsInf(value, 1) || math.IsInf(value, -1)
}

func calculatePercent(value1, value2 int64) int64 {
	return int64(float64(value1) / float64(value2) * 100)
}

func inThreshold(valuePercent int64) bool {
	return valuePercent >= 100-thresholdPercent && valuePercent <= 100+thresholdPercent
}
