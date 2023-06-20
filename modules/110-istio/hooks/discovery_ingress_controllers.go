/*
Copyright 2023 Flant JSC

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

type IstioIngressGatewayController struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        lib.Queue("istio-ingress-gateway-controller"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "controller",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "IngressIstioController",
			FilterFunc: applyDiscoveryIstioIngressControllerFilter,
		},
	},
}, setInternalIngressControllers)

func applyDiscoveryIstioIngressControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, _ := unstructured.NestedMap(obj.Object, "spec")
	if !ok {
		return nil, fmt.Errorf("istio ingress gateway controller %s has no spec field", name)
	}
	return IstioIngressGatewayController{Name: name, Spec: spec}, nil
}

func setInternalIngressControllers(input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots["controller"]
	controllers := make([]IstioIngressGatewayController, 0, len(controllersFilterResult))

	for _, c := range controllersFilterResult {
		controller := c.(IstioIngressGatewayController)
		controllers = append(controllers, controller)
	}

	input.Values.Set("istio.internal.ingressControllers", controllers)
	return nil
}
