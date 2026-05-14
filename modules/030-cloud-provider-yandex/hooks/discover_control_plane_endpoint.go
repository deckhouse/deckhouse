/*
Copyright 2026 Flant JSC

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
	"context"
	"fmt"
	"net"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ingressControlPlaneEndpoint struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

func applyKubernetesAPIIngressFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ingress := &networkingv1.Ingress{}
	if err := sdk.FromUnstructured(obj, ingress); err != nil {
		return nil, fmt.Errorf("cannot convert ingress: %w", err)
	}

	for _, rule := range ingress.Spec.Rules {
		if rule.Host != "" {
			return ingressControlPlaneEndpoint{Host: rule.Host, Port: 443}, nil
		}
	}

	return ingressControlPlaneEndpoint{}, nil
}

type serviceControlPlaneEndpoint struct {
	Host string `json:"host"`
	Port int32  `json:"port"`
}

func applyAPILoadBalancerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	if err := sdk.FromUnstructured(obj, service); err != nil {
		return nil, fmt.Errorf("cannot convert service: %w", err)
	}

	var port int32
	if len(service.Spec.Ports) > 0 {
		port = service.Spec.Ports[0].Port
	}

	for _, ingress := range service.Status.LoadBalancer.Ingress {
		if ingress.Hostname != "" {
			return serviceControlPlaneEndpoint{Host: ingress.Hostname, Port: port}, nil
		}
		if ingress.IP != "" {
			return serviceControlPlaneEndpoint{Host: ingress.IP, Port: port}, nil
		}
	}

	return serviceControlPlaneEndpoint{Port: port}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "publish_api_ingress",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"kubernetes-api"}},
			FilterFunc:   applyKubernetesAPIIngressFilter,
		},
		{
			Name:       "publish_api_load_balancer",
			ApiVersion: "v1",
			Kind:       "Service",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-control-plane-apiserver"}},
			FilterFunc:   applyAPILoadBalancerFilter,
		},
	},
}, discoverControlPlaneEndpoint)

func discoverControlPlaneEndpoint(_ context.Context, input *go_hook.HookInput) error {
	const valuesPath = "cloudProviderYandex.internal.controlPlaneEndpoint"

	if len(input.Snapshots.Get("publish_api_ingress")) > 0 {
		var endpoint ingressControlPlaneEndpoint
		if err := input.Snapshots.Get("publish_api_ingress")[0].UnmarshalTo(&endpoint); err != nil {
			return fmt.Errorf("failed to unmarshal 'publish_api_ingress' snapshot: %w", err)
		}
		if endpoint.Host != "" && endpoint.Port > 0 {
			input.Values.Set(valuesPath+".host", endpoint.Host)
			input.Values.Set(valuesPath+".port", endpoint.Port)
			return nil
		}
	}

	if len(input.Snapshots.Get("publish_api_load_balancer")) > 0 {
		var endpoint serviceControlPlaneEndpoint
		if err := input.Snapshots.Get("publish_api_load_balancer")[0].UnmarshalTo(&endpoint); err != nil {
			return fmt.Errorf("failed to unmarshal 'publish_api_load_balancer' snapshot: %w", err)
		}
		if endpoint.Host != "" && endpoint.Port > 0 {
			input.Values.Set(valuesPath+".host", endpoint.Host)
			input.Values.Set(valuesPath+".port", int(endpoint.Port))
			return nil
		}
	}

	clusterMasterAddresses := input.Values.Get("nodeManager.internal.clusterMasterAddresses").Array()
	if len(clusterMasterAddresses) == 0 {
		input.Values.Remove(valuesPath)
		return nil
	}

	host, port, err := net.SplitHostPort(clusterMasterAddresses[0].String())
	if err != nil {
		return fmt.Errorf("failed to parse nodeManager.internal.clusterMasterAddresses[0]: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("failed to convert nodeManager.internal.clusterMasterAddresses[0] port to int: %w", err)
	}

	input.Values.Set(valuesPath+".host", host)
	input.Values.Set(valuesPath+".port", portNumber)
	return nil
}
