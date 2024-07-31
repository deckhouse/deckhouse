/*
Copyright 2024 Flant JSC

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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	systemRegistryProxyPort = 5001
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
	Queue:        "/modules/node-manager/discover-system-registry",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "system_registry_proxy",
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "system-registry",
					"tier": "control-plane",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: systemRegistryPodFilter,
		},
	},
}, handleSystemRegistryProxyEndpoints)

func systemRegistryPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	return fmt.Sprintf("%s:%d", pod.Status.HostIP, systemRegistryProxyPort), nil
}

func handleSystemRegistryProxyEndpoints(input *go_hook.HookInput) error {
	endpointsSet := set.NewFromSnapshot(input.Snapshots["system_registry_proxy"])
	endpointsList := endpointsSet.Slice() // sorted

	if len(endpointsList) == 0 {
		return nil
	}
	input.LogEntry.Infof("found system registry proxy endpoints: %v", endpointsList)
	input.Values.Set("nodeManager.internal.systemRegistryProxy.addresses", endpointsList)
	return nil
}
