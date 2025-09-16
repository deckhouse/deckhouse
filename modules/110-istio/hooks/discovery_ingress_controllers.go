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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

func setInternalIngressControllers(_ context.Context, input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots.Get("controller")
	controllers := make([]IstioIngressGatewayController, 0, len(controllersFilterResult))

	for controller, err := range sdkobjectpatch.SnapshotIter[IstioIngressGatewayController](controllersFilterResult) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'controller' snapshot: %w", err)
		}

		controllers = append(controllers, controller)
	}

	input.Values.Set("istio.internal.ingressControllers", controllers)
	return nil
}
