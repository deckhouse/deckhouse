package hooks

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	autoscaler "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

const (
	groupLabelKey  = "workload-resource-policy.deckhouse.io"
	everyNodeLabel = "every-node"
	masterLabel    = "master"
)

func applyVpaResourcesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	v := &autoscaler.VerticalPodAutoscaler{}
	err := sdk.FromUnstructured(obj, v)
	if err != nil {
		return nil, fmt.Errorf("cannot parse vpa object from unstructured: %v", err)
	}

	if v.Labels[groupLabelKey] != everyNodeLabel && v.Labels[groupLabelKey] != masterLabel {
		return nil, nil
	}
	return v, nil
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
		Schedule: []go_hook.ScheduleConfig{
			{Name: "vpaCron", Crontab: "0 */6 * * *"},
		},
		Queue: "/modules/vertical-pod-autoscaler",
		Settings: &go_hook.HookConfigSettings{
			ExecutionMinInterval: 15 * time.Minute,
			ExecutionBurst:       1,
		},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "Vpa",
				ApiVersion: "autoscaling.k8s.io/v1",
				Kind:       "VerticalPodAutoscaler",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"heritage": "deckhouse",
					},
				},
				FilterFunc: applyVpaResourcesFilter,
			},
		},
	}, updateVpaResources)
)

func updateVpaResources(input *go_hook.HookInput) error {

	var (
		configEveryNodeMilliCPU  int64
		configEveryNodeMemory    int64
		configMasterNodeMilliCPU int64
		configMasterNodeMemory   int64

		totalRequestsMasterNodeMilliCPU int64
		totalRequestsMasterNodeMemory   int64
		totalRequestsEveryNodeMilliCPU  int64
		totalRequestsEveryNodeMemory    int64
	)

	configEveryNodeMilliCPU, err := getPathInt(input, "global.modules.resourcesRequests.internal.milliCpuEveryNode")
	if err != nil {
		return err
	}
	configEveryNodeMemory, err = getPathInt(input, "global.modules.resourcesRequests.internal.memoryEveryNode")
	if err != nil {
		return err
	}
	configMasterNodeMilliCPU, err = getPathInt(input, "global.modules.resourcesRequests.internal.milliCpuMaster")
	if err != nil {
		return err
	}
	configMasterNodeMemory, err = getPathInt(input, "global.modules.resourcesRequests.internal.memoryMaster")
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
		v := snapshot.(*autoscaler.VerticalPodAutoscaler)
		if v.Status.Recommendation == nil {
			continue
		}

		for _, r := range v.Status.Recommendation.ContainerRecommendations {
			ut := r.UncappedTarget
			switch v.Labels[groupLabelKey] {
			case masterLabel:
				totalRequestsMasterNodeMilliCPU += ut.Cpu().MilliValue()
				totalRequestsMasterNodeMemory += ut.Memory().Value()
			case everyNodeLabel:
				totalRequestsEveryNodeMilliCPU += ut.Cpu().MilliValue()
				totalRequestsEveryNodeMemory += ut.Memory().Value()
			}
		}
	}

	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		v := snapshot.(*autoscaler.VerticalPodAutoscaler)
		if v.Status.Recommendation == nil {
			continue
		}
		err = input.ObjectPatcher.FilterObject(func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			var (
				recommendationsMilliCPU int64
				recommendationsMemory   int64
				containerPolicies       []autoscaler.ContainerResourcePolicy
			)

			v := &autoscaler.VerticalPodAutoscaler{}
			err := sdk.FromUnstructured(obj, v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse vpa object from unstructured: %v", err)
			}

			for _, container := range v.Status.Recommendation.ContainerRecommendations {
				switch v.Labels[groupLabelKey] {
				case masterLabel:
					recommendationsMilliCPU = container.UncappedTarget.Cpu().MilliValue() * configMasterNodeMilliCPU / totalRequestsMasterNodeMilliCPU
					recommendationsMemory = container.UncappedTarget.Memory().Value() * configMasterNodeMemory / totalRequestsMasterNodeMemory
				case everyNodeLabel:
					recommendationsMilliCPU = container.UncappedTarget.Cpu().MilliValue() * configEveryNodeMilliCPU / totalRequestsEveryNodeMilliCPU
					recommendationsMemory = container.UncappedTarget.Memory().Value() * configEveryNodeMemory / totalRequestsEveryNodeMemory
				}
				newContainerPolicy := autoscaler.ContainerResourcePolicy{ContainerName: container.ContainerName}
				for _, cp := range v.Spec.ResourcePolicy.ContainerPolicies {
					if cp.ContainerName == container.ContainerName {
						newContainerPolicy = cp
						break
					}
				}
				newContainerPolicy.MaxAllowed = v1.ResourceList{
					v1.ResourceCPU:    *resource.NewMilliQuantity(recommendationsMilliCPU, resource.BinarySI),
					v1.ResourceMemory: *resource.NewQuantity(recommendationsMemory, resource.DecimalExponent),
				}
				containerPolicies = append(containerPolicies, newContainerPolicy)
			}
			v.Spec.ResourcePolicy.ContainerPolicies = containerPolicies

			result, err := sdk.ToUnstructured(v)
			if err != nil {
				return nil, fmt.Errorf("cannot parse unstructured to object: %v", err)
			}
			return result, nil
		}, v.APIVersion, v.Kind, v.Namespace, v.Name, "")

		if err != nil {
			return err
		}
	}

	return nil
}

func getPathInt(input *go_hook.HookInput, path string) (int64, error) {
	if !input.Values.Exists(path) {
		return 0, fmt.Errorf("%s must be set", path)
	}
	return input.Values.Get(path).Int(), nil
}
