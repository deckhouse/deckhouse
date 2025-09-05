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

package change_host_address

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const initialHostAddressAnnotation = "node.deckhouse.io/initial-host-ip"

type address struct {
	Name        string
	Host        string
	InitialHost string
}

func getAddress(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &v1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot convert pod: %v", err)
	}

	return address{
		Name:        pod.Name,
		Host:        pod.Status.HostIP,
		InitialHost: pod.Annotations[initialHostAddressAnnotation],
	}, nil
}

func RegisterHook(appName, namespace string) bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "pod",
				ApiVersion: "v1",
				Kind:       "Pod",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{namespace},
					},
				},
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "app",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{appName},
						},
					},
				},
				FilterFunc: getAddress,
			},
		},
	}, wrapChangeAddressHandler(namespace))
}

func wrapChangeAddressHandler(namespace string) func(_ context.Context, input *go_hook.HookInput) error {
	return func(_ context.Context, input *go_hook.HookInput) error {
		return changeHostAddressHandler(namespace, input)
	}
}

func changeHostAddressHandler(namespace string, input *go_hook.HookInput) error {
	pods := input.Snapshots.Get("pod")
	if len(pods) == 0 {
		return nil
	}

	addresses, err := sdkobjectpatch.UnmarshalToStruct[address](input.Snapshots, "pod")
	if err != nil {
		return fmt.Errorf("cannot unmarshal pods: %v", err)
	}

	for _, podAddress := range addresses {
		if podAddress.Host == "" {
			// Pod doesn't exist, we can skip it
			continue
		}

		if podAddress.InitialHost == "" {
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						initialHostAddressAnnotation: podAddress.Host,
					},
				},
			}
			input.PatchCollector.PatchWithMerge(patch, "v1", "Pod", namespace, podAddress.Name)
			continue
		}

		if podAddress.InitialHost != podAddress.Host {
			input.PatchCollector.Delete("v1", "Pod", namespace, podAddress.Name)
		}
	}

	return nil
}
