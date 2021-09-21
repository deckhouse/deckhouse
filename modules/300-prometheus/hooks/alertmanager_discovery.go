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
	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type AlertmanagerService struct {
	Prometheus string                  `json:"prometheus"`
	Service    AlertmanagerServiceInfo `json:"service"`
}

type AlertmanagerServiceInfo struct {
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	PathPrefix string      `json:"pathPrefix"`
	Port       interface{} `json:"port"`
}

func applyAlertmanagerServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, err
	}

	as := &AlertmanagerService{}

	as.Prometheus = service.ObjectMeta.Labels["prometheus.deckhouse.io/alertmanager"]
	as.Service.Namespace = service.ObjectMeta.Namespace
	as.Service.Name = service.ObjectMeta.Name

	switch {
	case len(service.Spec.Ports[0].Name) != 0:
		as.Service.Port = service.Spec.Ports[0].Name
	case service.Spec.Ports[0].Port != 0:
		as.Service.Port = service.Spec.Ports[0].Port
	default:
		return nil, spew.Errorf("Can't find Name or Port in the first port of a Service %#+v", as.Service)
	}

	as.Service.PathPrefix = "/"
	if prefix, ok := service.ObjectMeta.Annotations["prometheus.deckhouse.io/alertmanager-path-prefix"]; ok {
		as.Service.PathPrefix = prefix
	}

	return as, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "alertmanager_services",
			ApiVersion: "v1",
			Kind:       "Service",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "prometheus.deckhouse.io/alertmanager",
					Operator: "Exists",
				},
			}},
			FilterFunc: applyAlertmanagerServiceFilter,
		},
	},
}, alertManagerHandler)

func alertManagerHandler(input *go_hook.HookInput) error {
	snaps, ok := input.Snapshots["alertmanager_services"]
	if !ok {
		input.LogEntry.Info("No AlertManager Services received, skipping setting values")
		return nil
	}

	alertManagers := map[string][]AlertmanagerServiceInfo{}
	for _, svc := range snaps {
		alertManagerService := svc.(*AlertmanagerService)

		if _, ok := alertManagers[alertManagerService.Prometheus]; !ok {
			alertManagers[alertManagerService.Prometheus] = make([]AlertmanagerServiceInfo, 0)
		}
		alertManagers[alertManagerService.Prometheus] = append(alertManagers[alertManagerService.Prometheus], alertManagerService.Service)
	}

	input.Values.Set("prometheus.internal.alertmanagers", alertManagers)

	return nil
}
