/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type IstioIngressGatewayController struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        internal.Queue("istio-ingress-gateway"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "controller",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IngressIstioController",
			FilterFunc: applyDiscoveryIngressControllerFilter,
		},
	},
}, setInternalValues)

func applyDiscoveryIngressControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from istio ingress gateway controller %s: %v", name, err)
	}
	if !ok {
		return nil, fmt.Errorf("istio ingress gateway controller %s has no spec field", name)
	}
	return IstioIngressGatewayController{Name: name, Spec: spec}, nil
}

func setInternalValues(input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots["controller"]
	var controllers []IstioIngressGatewayController

	for _, c := range controllersFilterResult {
		controller := c.(IstioIngressGatewayController)
		controllers = append(controllers, controller)
	}

	input.Values.Set("istio.internal.ingressControllers", controllers)
	return nil
}
