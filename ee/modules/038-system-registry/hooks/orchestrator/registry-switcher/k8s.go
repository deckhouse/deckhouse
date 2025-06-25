/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registryswitcher

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
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
	if err != nil {
		return Inputs{}, err
	}

	if !deckhousePod.IsExist {
		return Inputs{
			DeckhousePod: DeckhousePodStatus{
				IsExist:  false,
				IsReady:  false,
				ReadyMsg: "No Deckhouse leader pod found",
			},
		}, nil
	}
	return Inputs{DeckhousePod: deckhousePod}, err
}
