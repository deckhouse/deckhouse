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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Controller struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/ingress-nginx",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "controller",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "IngressNginxController",
			FilterFunc: applyControllerFilter,
		},
	},
}, setInternalValues)

func applyControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from ingress controller %s: %v", name, err)
	}
	if !ok {
		return nil, fmt.Errorf("ingress controller %s has no spec field", name)
	}

	// Set default values in order to save compatibility
	setDefaultEmptyObject("config", spec)

	inlet, _, err := unstructured.NestedString(spec, "inlet")
	if err != nil {
		return nil, fmt.Errorf("cannot get inlet from ingress controller spec: %v", err)
	}

	setDefaultEmptyObjectOnCondition("loadBalancer", spec, inlet == "LoadBalancer")
	setDefaultEmptyObjectOnCondition("loadBalancerWithProxyProtocol", spec, inlet == "LoadBalancerWithProxyProtocol")
	setDefaultEmptyObjectOnCondition("hostPort", spec, inlet == "HostPort")
	setDefaultEmptyObjectOnCondition("hostPortWithProxyProtocol", spec, inlet == "HostPortWithProxyProtocol")
	setDefaultEmptyObjectOnCondition("hostWithFailover", spec, inlet == "HostWithFailover")

	setDefaultEmptyObject("hstsOptions", spec)
	setDefaultEmptyObject("geoIP2", spec)
	setDefaultEmptyObject("resourcesRequests", spec)

	mode, _, err := unstructured.NestedString(spec, "resourcesRequests", "mode")
	if err != nil {
		return nil, fmt.Errorf("cannot get resourcesRequests.mode from ingress controller spec: %v", err)
	}

	if mode == "" {
		err := unstructured.SetNestedField(spec, "VPA", "resourcesRequests", "mode")
		if err != nil {
			return nil, fmt.Errorf("cannot set resourcesRequests.mode from ingress controller spec: %v", err)
		}
	}

	resourcesRequests, _, err := unstructured.NestedMap(spec, "resourcesRequests")
	if err != nil {
		return nil, fmt.Errorf("cannot get resourcesRequests from ingress controller spec: %v", err)
	}

	setDefaultEmptyObject("static", resourcesRequests)
	setDefaultEmptyObject("vpa", resourcesRequests)

	vpa, _, err := unstructured.NestedMap(resourcesRequests, "vpa")
	if err != nil {
		return nil, fmt.Errorf("cannot get resourcesRequests.vpa from ingress controller spec: %v", err)
	}

	setDefaultEmptyObject("cpu", vpa)
	setDefaultEmptyObject("memory", vpa)

	err = unstructured.SetNestedMap(resourcesRequests, vpa, "vpa")
	if err != nil {
		return nil, fmt.Errorf("cannot set resourcesRequests.vpa from ingress controller spec: %v", err)
	}

	err = unstructured.SetNestedMap(spec, resourcesRequests, "resourcesRequests")
	if err != nil {
		return nil, fmt.Errorf("cannot set resourcesRequests from ingress controller spec: %v", err)
	}
	return Controller{Name: name, Spec: spec}, nil
}

func setDefaultEmptyObject(key string, obj map[string]interface{}) {
	if _, ok := obj[key]; !ok {
		obj[key] = make(map[string]interface{})
	}
}

func setDefaultEmptyObjectOnCondition(key string, obj map[string]interface{}, condition bool) {
	if condition {
		setDefaultEmptyObject(key, obj)
	} else {
		obj[key] = make(map[string]interface{})
	}
}

func setInternalValues(input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots["controller"]
	defaultControllerVersion := input.Values.Get("ingressNginx.defaultControllerVersion").String()
	input.MetricsCollector.Expire("")

	var controllers []Controller

	for _, c := range controllersFilterResult {
		controller := c.(Controller)

		version, found, err := unstructured.NestedString(controller.Spec, "controllerVersion")
		if err != nil {
			return fmt.Errorf("cannot get controllerVersion from ingress controller spec: %v", err)
		}
		if len(version) == 0 || !found {
			// we shouldn't inject default version to spec, because all templates are following the next logic:
			// {{- $controllerVersion := $crd.spec.controllerVersion | default $context.Values.ingressNginx.defaultControllerVersion }}
			// controllerVersion should be absent if not specified explicitly
			version = defaultControllerVersion // it's used only for metrics
		}
		controllers = append(controllers, controller)

		input.MetricsCollector.Set("d8_ingress_nginx_controller", 1, map[string]string{
			"controller_name":    controller.Name,
			"controller_version": version,
		})
	}

	input.Values.Set("ingressNginx.internal.ingressControllers", controllers)

	return nil
}
