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

func (*KubernetesServicePort) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := go_hook.ConvertUnstructured(obj, service)
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

func (*KubernetesEndpoints) ApplyFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	endpoints := &v1.Endpoints{}
	err := go_hook.ConvertUnstructured(obj, endpoints)
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
	Queue: "/modules/user-authn",
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
			Filterable: &KubernetesServicePort{},
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
			Filterable: &KubernetesEndpoints{},
		},
	},
}, discoverApiserverEndpoints)

func discoverApiserverEndpoints(input *go_hook.HookInput) error {
	const (
		addressesPath  = "userAuthn.internal.kubernetesApiserverAddresses"
		targetPortPath = "userAuthn.internal.kubernetesApiserverTargetPort"
	)

	publishAPIEnabled := input.Values.Values.Path("userAuthn.publishAPI.enable").Data().(bool)
	if !publishAPIEnabled {
		if input.Values.Values.ExistsP(addressesPath) {
			input.Values.Remove(addressesPath)
		}

		if input.Values.Values.Exists(targetPortPath) {
			input.Values.Remove(targetPortPath)
		}
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
