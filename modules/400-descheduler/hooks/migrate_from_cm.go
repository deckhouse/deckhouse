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
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/mitchellh/mapstructure"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/deckhouse/deckhouse/modules/400-descheduler/hooks/internal/api/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 500},
}, createFirstDeschedulerCR)

func createFirstDeschedulerCR(input *go_hook.HookInput) error {
	config, ok := input.ConfigValues.GetOk("descheduler")
	if !ok || len(config.Map()) == 0 {
		return nil
	}

	deschedulerCR := &v1alpha1.Descheduler{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Descheduler",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Spec: v1alpha1.DeschedulerSpec{},
	}

	if value := config.Get("removeDuplicates"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemoveDuplicates = &v1alpha1.RemoveDuplicates{}
	}
	if value := config.Get("lowNodeUtilization"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.LowNodeUtilization = &v1alpha1.LowNodeUtilization{
			Params: &v1alpha1.LowNodeUtilizationParams{
				NodeResourceUtilizationThresholds: &v1alpha1.NodeResourceUtilizationThresholdsFiltering{
					Thresholds: map[v1.ResourceName]v1alpha1.Percentage{
						"cpu":    40,
						"memory": 50,
						"pods":   40,
					},
					TargetThresholds: map[v1.ResourceName]v1alpha1.Percentage{
						"cpu":    80,
						"memory": 90,
						"pods":   90,
					},
				},
			},
		}
	}
	if value := config.Get("highNodeUtilization"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.HighNodeUtilization = &v1alpha1.HighNodeUtilization{
			Params: &v1alpha1.HighNodeUtilizationParams{
				NodeResourceUtilizationThresholds: &v1alpha1.NodeResourceUtilizationThresholdsFiltering{
					Thresholds: map[v1.ResourceName]v1alpha1.Percentage{
						"cpu":    50,
						"memory": 50,
					},
				},
			},
		}
	}
	if value := config.Get("removePodsViolatingInterPodAntiAffinity"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemovePodsViolatingInterPodAntiAffinity = &v1alpha1.RemovePodsViolatingInterPodAntiAffinity{}
	}
	if value := config.Get("removePodsViolatingNodeAffinity"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemovePodsViolatingNodeAffinity = &v1alpha1.RemovePodsViolatingNodeAffinity{
			Params: &v1alpha1.RemovePodsViolatingNodeAffinityParams{
				NodeAffinityType: []string{"requiredDuringSchedulingIgnoredDuringExecution"},
			},
		}
	}
	if value := config.Get("removePodsViolatingNodeTaints"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemovePodsViolatingNodeTaints = &v1alpha1.RemovePodsViolatingNodeTaints{}
	}
	if value := config.Get("removePodsViolatingTopologySpreadConstraint"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemovePodsViolatingTopologySpreadConstraint = &v1alpha1.RemovePodsViolatingTopologySpreadConstraint{}
	}
	if value := config.Get("removePodsHavingTooManyRestarts"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.RemovePodsHavingTooManyRestarts = &v1alpha1.RemovePodsHavingTooManyRestarts{
			Params: &v1alpha1.RemovePodsHavingTooManyRestartsParams{
				PodsHavingTooManyRestarts: &v1alpha1.PodsHavingTooManyRestartsParameters{
					PodRestartThreshold:     100,
					IncludingInitContainers: true,
				},
			},
		}
	}
	if value := config.Get("podLifeTime"); value.Exists() {
		deschedulerCR.Spec.DeschedulerPolicy.Strategies.PodLifeTime = &v1alpha1.PodLifeTime{
			Params: &v1alpha1.PodLifeTimeParams{
				PodLifeTime: &v1alpha1.PodLifeTimeParameters{
					MaxPodLifeTimeSeconds: uintPtr(86400),
					PodStatusPhases:       []string{"Pending"},
				},
			},
		}
	}

	if value := config.Get("nodeSelector"); value.Exists() {
		rawSelectorMap := value.Map()
		nodeSelectorSet := make(labels.Set, len(rawSelectorMap))
		for k, v := range rawSelectorMap {
			nodeSelectorSet[k] = v.String()
		}

		deschedulerCR.Spec.DeploymentTemplate.NodeSelector = nodeSelectorSet
	}

	if value := config.Get("tolerations"); value.Exists() {
		var tolerations []v1.Toleration

		err := mapstructure.Decode(value.Value(), &tolerations)
		if err != nil {
			return fmt.Errorf("can't decode existing tolerations %+v: %s", value.Value(), err)
		}

		deschedulerCR.Spec.DeploymentTemplate.Tolerations = tolerations
	}

	input.PatchCollector.Create(deschedulerCR, object_patch.UpdateIfExists())

	input.ConfigValues.Set("descheduler", map[string]string{})

	return nil
}

func uintPtr(u uint) *uint {
	return &u
}
