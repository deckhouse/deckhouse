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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "kube_api_ep",
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

type apiEndpoints struct {
	HostPort []string
}

func apiEndpointsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var endpoint corev1.Endpoints
	err := sdk.FromUnstructured(obj, &endpoint)
	if err != nil {
		return nil, err
	}

	var apiEP apiEndpoints

	for _, subset := range endpoint.Subsets {
		for _, address := range subset.Addresses {
			ip := address.IP
			for _, port := range subset.Ports {
				apiEP.HostPort = append(apiEP.HostPort, fmt.Sprintf("%s:%d", ip, port.Port))
			}
		}
	}

	return apiEP, nil
}

func handleAPIEndpoints(input *go_hook.HookInput) error {
	snap := input.Snapshots["kube_api_ep"]
	if len(snap) == 0 {
		input.LogEntry.Error("kubernetes endpoints not found")
		return nil
	}

	apiEndpoints := snap[0].(apiEndpoints)

	if len(apiEndpoints.HostPort) == 0 {
		return errors.New("no kubernetes apiserver endpoints host:port specified")
	}

	input.Values.Set("nodeManager.internal.clusterMasterAddresses", apiEndpoints.HostPort)

	return nil
}
