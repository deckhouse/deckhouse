/*
Copyright 2025 Flant JSC

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

package registryswitcher

import (
	"errors"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/helpers"
)

const (
	registryVersionAnnotation = "checksum/registry-version"
)

func KubernetesConfig(name string) go_hook.KubernetesConfig {
	return go_hook.KubernetesConfig{
		Name:              name,
		ApiVersion:        "v1",
		Kind:              "Pod",
		NamespaceSelector: helpers.NamespaceSelector,
		LabelSelector: &v1.LabelSelector{
			MatchLabels: map[string]string{
				"app":    "deckhouse",
				"leader": "true",
			},
		},
		FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
			var pod corev1.Pod
			if err := sdk.FromUnstructured(obj, &pod); err != nil {
				return nil, fmt.Errorf("failed to convert deckhouse pod to struct: %w", err)
			}

			IsReady := false
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
					IsReady = true
				}
			}

			ReadyMsg := "Pod is ready"
			if !IsReady {
				ReadyMsg = fmt.Sprintf("Pod phase is %s", pod.Status.Phase)
			}

			return DeckhousePodStatus{
				IsExist:         true,
				IsReady:         IsReady,
				ReadyMsg:        ReadyMsg,
				RegistryVersion: pod.Annotations[registryVersionAnnotation],
			}, nil
		},
	}
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	deckhousePod, err := helpers.SnapshotToSingle[DeckhousePodStatus](input, name)
	if err != nil && !errors.Is(err, helpers.ErrNoSnapshot) {
		return Inputs{}, err
	}

	if !deckhousePod.IsExist {
		deckhousePod = DeckhousePodStatus{
			IsExist:  false,
			IsReady:  false,
			ReadyMsg: "No Deckhouse leader pod found",
		}
	}
	return Inputs{DeckhousePod: deckhousePod}, err
}
