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
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const apiserverPort = 6443

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_apiserver",
			ApiVersion: "v1",
			Kind:       "Pod",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "kube-apiserver",
					"tier":      "control-plane",
				},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: apiserverPodFilter,
		},
		{
			Name:       "apiserver_endpoints",
			ApiVersion: "v1",
			Kind:       "Endpoints",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"default"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes"},
			},
			FilterFunc: apiEndpointsFilter,
		},
	},
}, handleAPIEndpoints)

func apiserverPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var isReady bool

	pod := &corev1.Pod{}
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pod object from unstructured: %v", err)
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			isReady = true
			break
		}
	}
	if !isReady {
		return "", nil
	}
	return fmt.Sprintf("%s:%d", pod.Status.PodIP, apiserverPort), nil
}

func apiEndpointsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var endpoints corev1.Endpoints

	err := sdk.FromUnstructured(obj, &endpoints)
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0)

	for _, s := range endpoints.Subsets {
		ports := make([]int32, 0)
		for _, port := range s.Ports {
			if port.Name == "https" {
				ports = append(ports, port.Port)
			}
		}

		for _, addrObj := range s.Addresses {
			for _, port := range ports {
				addr := net.JoinHostPort(addrObj.IP, strconv.Itoa(int(port)))
				addresses = append(addresses, addr)
			}
		}
	}
	return addresses, nil
}

func handleAPIEndpoints(input *go_hook.HookInput) error {
	endpointsSet := set.NewFromSnapshot(input.Snapshots["kube_apiserver"])

	for _, ep := range input.Snapshots["apiserver_endpoints"] {
		endpointsSet.Add(ep.([]string)...)
	}
	endpointsSet.Delete("") // clean faulty pods

	endpointsList := endpointsSet.Slice() // sorted

	if len(endpointsList) == 0 {
		return errors.New("no kubernetes apiserver endpoints host:port specified")
	}

	input.Values.Set("nodeManager.internal.clusterMasterAddresses", endpointsList)

	return nil
}
