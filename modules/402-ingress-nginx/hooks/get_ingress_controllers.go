/*
Copyright 2021 Flant CJSC

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
	"os/exec"
	"strings"

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

func cidrToRegex(cidr string) (string, error) {
	cmd := exec.Command("rgxg", "cidr", cidr)

	stdout, err := cmd.CombinedOutput()
	stdoutTrimmed := strings.TrimSuffix(string(stdout), "\n")
	if err != nil {
		return stdoutTrimmed, err
	}

	return stdoutTrimmed, nil
}

func applyControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from ingress controller %s: %v", name, err)
	}
	if !ok {
		return nil, fmt.Errorf("ingress controller %s has no spec field", name)
	}

	acceptRequestsFromCIDRs, _, err := unstructured.NestedStringSlice(spec, "acceptRequestsFrom")
	if err != nil {
		return nil, fmt.Errorf("cannot get acceptRequestsFrom from ingress controller spec: %v", err)
	}

	var acceptRequestsFromRegexes []string
	for _, acceptFromCidr := range acceptRequestsFromCIDRs {
		rgxp, err := cidrToRegex(acceptFromCidr)
		if err != nil {
			return nil, fmt.Errorf("error run rgxg: %v", err)
		}
		acceptRequestsFromRegexes = append(acceptRequestsFromRegexes, "~"+strings.ReplaceAll(rgxp, "\\", ""))
	}

	if len(acceptRequestsFromRegexes) > 0 {
		err := unstructured.SetNestedStringSlice(spec, acceptRequestsFromRegexes, "acceptRequestsFrom")
		if err != nil {
			return nil, fmt.Errorf("cannot set acceptRequestsFrom for ingress controller spec: %v", err)
		}
	}

	return Controller{Name: name, Spec: spec}, nil
}

func setInternalValues(input *go_hook.HookInput) error {
	controllersFilterResult := input.Snapshots["controller"]
	defaultControllerVersion := input.Values.Get("ingressNginx.defaultControllerVersion").String()

	var controllers []Controller

	for _, c := range controllersFilterResult {
		controller := c.(Controller)

		version, found, err := unstructured.NestedString(controller.Spec, "controllerVersion")
		if err != nil {
			return fmt.Errorf("cannot get controllerVersion from ingress controller spec: %v", err)
		}
		if len(version) == 0 || !found {
			err := unstructured.SetNestedField(controller.Spec, defaultControllerVersion, "controllerVersion")
			if err != nil {
				return fmt.Errorf("cannot set controllerVersion for ingress controller spec: %v", err)
			}
		}
		controllers = append(controllers, controller)
	}

	input.Values.Set("ingressNginx.internal.ingressControllers", controllers)

	return nil
}
