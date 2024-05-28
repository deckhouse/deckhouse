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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type KubernetesServicePort intstr.IntOrString

func applyKubernetesServicePortFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes service to service: %v", err)
	}

	ports := service.Spec.Ports
	if len(ports) != 1 {
		return nil, fmt.Errorf("expected only one port for kubernetes service, got: %v", ports)
	}

	return ports[0].TargetPort.IntVal, nil
}

type KubernetesEndpoints []string

func applyKubernetesEndpointsFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	endpoints := &v1.Endpoints{}
	err := sdk.FromUnstructured(obj, endpoints)
	if err != nil {
		return nil, fmt.Errorf("cannot convert kubernetes service endpoints to endpoints: %v", err)
	}

	var parsedEndpoints KubernetesEndpoints
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			parsedEndpoints = append(parsedEndpoints, address.IP)
		}
	}

	return parsedEndpoints, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/user-authn",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "port",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"default"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubernetes"},
			},
			FilterFunc: applyKubernetesServicePortFilter,
		},
		{
			Name:       "endpoints",
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
			FilterFunc: applyKubernetesEndpointsFilter,
		},
	},
}, discoverApiserverEndpoints)

func discoverApiserverEndpoints(input *go_hook.HookInput) error {
	const (
		addressesPath  = "userAuthn.internal.kubernetesApiserverAddresses"
		targetPortPath = "userAuthn.internal.kubernetesApiserverTargetPort"
	)

	publishAPIEnabled := input.Values.Get("userAuthn.publishAPI.enabled").Bool()
	if !publishAPIEnabled {
		input.Values.Remove(addressesPath)
		input.Values.Remove(targetPortPath)
		return nil
	}

	ports := input.Snapshots["port"]
	if len(ports) == 0 {
		return fmt.Errorf("kubernetes service pod was not discovered")
	}

	endpoints := input.Snapshots["endpoints"]
	if len(endpoints) == 0 {
		return fmt.Errorf("kubernetes service endpoints was not discovered")
	}

	input.Values.Set(targetPortPath, ports[0])
	input.Values.Set(addressesPath, endpoints[0])
	return nil
}
