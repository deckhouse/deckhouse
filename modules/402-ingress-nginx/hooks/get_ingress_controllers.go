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
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
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

	// If deletion timestamp exists — skip controller to force helm deleting the resources by excluding the controller from "ingressNginx.internal.ingressControllers".
	// need for handle_finalizers hook proper work
	if obj.GetDeletionTimestamp() != nil {
		return nil, nil
	}

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

	logLevel, found, err := unstructured.NestedString(spec, "controllerLogLevel")
	if err != nil {
		return nil, fmt.Errorf("cannot get controllerLogLevel from ingress controller spec: %v", err)
	}
	if !found || logLevel == "" {
		err = unstructured.SetNestedField(spec, "Info", "controllerLogLevel")
		if err != nil {
			return nil, fmt.Errorf("cannot set default controllerLogLevel in ingress controller spec: %v", err)
		}
	}

	// Set validationEnabled to false if suspended annotation is present
	metadata, _, err := unstructured.NestedMap(obj.Object, "metadata")
	if err != nil {
		return nil, fmt.Errorf("cannot get metadata from ingress controller: %v", err)
	}
	annotationsRaw, ok := metadata["annotations"]
	if ok && annotationsRaw != nil {
		annotations, ok := annotationsRaw.(map[string]interface{})
		if ok {
			if _, hasAnnotation := annotations[internal.IngressNginxControllerSuspendAnnotation]; hasAnnotation {
				spec["validationEnabled"] = false
			}
		}
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

func setInternalValues(_ context.Context, input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots.Get("controller")
	defaultControllerVersion := input.Values.Get("ingressNginx.defaultControllerVersion").String()
	input.MetricsCollector.Expire("")

	controllers := make([]Controller, 0, len(controllersFilterResult))

	for controller, err := range sdkobjectpatch.SnapshotIter[Controller](controllersFilterResult) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'controller' snapshots: %w", err)
		}

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

		nginxEnabledMemoryProfiling, npeFound, err := unstructured.NestedBool(controller.Spec, "nginxProfilingEnabled")

		if err != nil {
			input.Logger.Error(fmt.Sprintf("cannot get nginxProfilingEnabled from ingress controller spec: %v", err))
			continue
		}

		if npeFound && nginxEnabledMemoryProfiling {
			input.MetricsCollector.Set("d8_ingress_nginx_controller_profiling_enabled", 1, map[string]string{
				"controller_name": controller.Name,
			})
		} else {
			input.MetricsCollector.Set("d8_ingress_nginx_controller_profiling_enabled", 0, map[string]string{
				"controller_name": controller.Name,
			})
		}

		// fire alert if maxmindAccountID not set.
		_, licFound, err := unstructured.NestedString(controller.Spec, "geoIP2", "maxmindLicenseKey")
		if err != nil {
			input.Logger.Error(fmt.Sprintf("cannot get maxmindLicenseKey from ingress controller spec.geoIP2: %v", err))
			continue
		}

		_, acFound, err := unstructured.NestedString(controller.Spec, "geoIP2", "maxmindAccountID")
		if err != nil {
			input.Logger.Error(fmt.Sprintf("cannot get maxmindAccountID from ingress controller spec.geoIP2: %v", err))
			continue
		}

		val := 0.0
		if licFound && !acFound {
			val = 1.0
		}
		input.MetricsCollector.Set("d8_ingress_nginx_controller_maxmind_account_id_not_set", val, map[string]string{
			"controller_name": controller.Name,
		})
	}

	input.Values.Set("ingressNginx.internal.ingressControllers", controllers)

	return nil
}
