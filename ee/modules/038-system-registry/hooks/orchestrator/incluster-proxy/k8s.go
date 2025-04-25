/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inclusterproxy

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/helpers"
)

const (
	podsSnapName = "pods"
)

func snapName(prefix, name string) string {
	return fmt.Sprintf("%s-->%s", prefix, name)
}

func KubernetsConfig(name string) []go_hook.KubernetesConfig {
	ret := []go_hook.KubernetesConfig{
		{
			Name:              snapName(name, podsSnapName),
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: helpers.NamespaceSelector,
			LabelSelector:     InClusterProxyPodsMatchLabels,
			FilterFunc: func(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
				var pod v1core.Pod

				err := sdk.FromUnstructured(obj, &pod)
				if err != nil {
					return nil, fmt.Errorf("failed to convert pod to struct: %v", err)
				}

				isReady := false
				for _, cond := range pod.Status.Conditions {
					if cond.Type == "Ready" && cond.Status == "True" {
						isReady = true
						break
					}
				}

				podObject := Pod{
					Ready:   isReady,
					Version: pod.Annotations[PodVersionAnnotation],
				}

				ret := helpers.NewKeyValue(pod.Name, podObject)
				return ret, nil
			},
		},
	}
	return ret
}

func InputsFromSnapshot(input *go_hook.HookInput, name string) (Inputs, error) {
	var (
		ret Inputs
		err error
	)

	pods, err := helpers.SnapshotToMap[string, Pod](input, snapName(name, podsSnapName))
	if err != nil {
		return ret, fmt.Errorf("get Pods snapshot error: %w", err)
	}
	ret.Pods = pods
	return ret, nil
}
