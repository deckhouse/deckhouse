/*
Copyright 2022 Flant JSC

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
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

// migration from 0.33 to 1.1 have to change ingress class but it's controller field is immutable
// we have to handle this situation manually - delete old IngressClass resources

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 30},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ingress_classes",
			ApiVersion:                   "networking.k8s.io/v1",
			Kind:                         "IngressClass",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   "ingress-nginx",
				}},
			FilterFunc: applyIngressClassFilter,
		},
	},
}, handleIngressClasses)

var (
	// edge version, with which we compare existed controllers
	edgeVersion = semver.MustParse("v1.0.0")
)

func applyIngressClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ic v1.IngressClass

	err := sdk.FromUnstructured(obj, &ic)
	if err != nil {
		return nil, err
	}

	return d8IngressClass{
		Name:       ic.Name,
		Controller: ic.Spec.Controller,
	}, nil
}

func handleIngressClasses(input *go_hook.HookInput) error {
	snap := input.Snapshots["ingress_classes"]
	if len(snap) == 0 {
		return nil
	}

	existingClasses := make(map[string]string, len(snap))
	for _, s := range snap {
		class := s.(d8IngressClass)
		existingClasses[class.Name] = class.Controller
	}

	conArray := input.Values.Get("ingressNginx.internal.ingressControllers").Array()

	for _, con := range conArray {
		var controller Controller

		err := json.Unmarshal([]byte(con.String()), &controller)
		if err != nil {
			return err
		}

		controllerName := "k8s.io/ingress-nginx"

		ingressClass, _, err := unstructured.NestedString(controller.Spec, "ingressClass")
		if err != nil {
			return err
		}

		existingController, ok := existingClasses[ingressClass]
		if !ok {
			continue
		}

		version, ok, err := unstructured.NestedString(controller.Spec, "controllerVersion")
		if err != nil {
			return err
		}
		if !ok {
			version = input.Values.Get("ingressNginx.defaultControllerVersion").String()
		}
		semV := semver.MustParse(version)
		if semV.GreaterThan(edgeVersion) {
			controllerName = fmt.Sprintf("ingress-nginx.deckhouse.io/%s", ingressClass)
		}

		if existingController != controllerName {
			input.PatchCollector.Delete("networking.k8s.io/v1", "IngressClass", "", ingressClass)
		}
	}

	return nil
}

type d8IngressClass struct {
	Name       string
	Controller string
}
