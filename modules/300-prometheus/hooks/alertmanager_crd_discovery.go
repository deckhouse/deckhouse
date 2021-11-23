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
	"errors"
	"fmt"
	"net/url"

	"github.com/davecgh/go-spew/spew"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "alertmanager_crds",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "CustomAlertmanager",
			FilterFunc: applyAlertmanagerCRDFilter,
		},
		{
			Name:                         "services",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnEvents:          pointer.BoolPtr(false),
			ExecuteHookOnSynchronization: pointer.BoolPtr(false),
			FilterFunc:                   applyAlertmanagerCRDServiceFilter,
		},
		{
			// deprecated way to set alertmanagers - through the labeled service
			Name:       "alertmanager_services",
			ApiVersion: "v1",
			Kind:       "Service",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "prometheus.deckhouse.io/alertmanager",
					Operator: "Exists",
				},
			}},
			FilterFunc: applyDeprecatedAlertmanagerServiceFilter,
		},
	},
}, crdAlertmanagerHandler)

type Alertmanager struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

type alertmanagerCRDService struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Port      interface{} `json:"port"`
}

func applyAlertmanagerCRDServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc corev1.Service
	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, err
	}

	crdService := alertmanagerCRDService{
		Name:      svc.Name,
		Namespace: svc.Namespace,
	}

	if len(svc.Spec.Ports) > 0 {
		switch {
		case len(svc.Spec.Ports[0].Name) != 0:
			crdService.Port = svc.Spec.Ports[0].Name
		case svc.Spec.Ports[0].Port != 0:
			crdService.Port = svc.Spec.Ports[0].Port
		default:
			return nil, spew.Errorf("Can't find Name or Port in the first port of a Service %#+v", svc)
		}
	}

	return crdService, nil
}

func applyAlertmanagerCRDFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from Alertmanager %s: %v", name, err)
	}
	if !ok {
		return nil, fmt.Errorf("alertmanager %s has no spec field", name)
	}

	return Alertmanager{Name: name, Spec: spec}, nil
}

func crdAlertmanagerHandler(input *go_hook.HookInput) error {
	snap := input.Snapshots["alertmanager_crds"]

	result := make([]alertmanagerValue, 0, len(snap))

	serviceDeclaredAlertmanagers := make(map[string][]alertmanagerServiceInfo)

	for _, s := range snap {
		am := s.(Alertmanager)

		address, _, _ := unstructured.NestedString(am.Spec, "external", "address")
		if address != "" {
			// parse static_sd_config with direct target
			value, err := parseTargetCR(am)
			if err != nil {
				return err
			}
			result = append(result, value)
		} else {
			// parse service
			old, err := parseServiceCR(am, input.Snapshots["services"])
			if err != nil {
				return err
			}
			if _, ok := serviceDeclaredAlertmanagers["main"]; !ok {
				serviceDeclaredAlertmanagers["main"] = make([]alertmanagerServiceInfo, 0)
			}
			serviceDeclaredAlertmanagers["main"] = append(serviceDeclaredAlertmanagers["main"], old)
		}
	}

	if len(result) > 0 {
		input.Values.Set("prometheus.internal.alerting.alertmanagers", result)
	} else {
		input.Values.Remove("prometheus.internal.alerting.alertmanagers")
	}

	// service discovery through the deprecated labeled services
	deprecatedServices := handleDeperecatedAlertmanagerServices(input)
	if len(deprecatedServices) > 0 {
		// merge old - service discovery AlertManagers and new CR
		for gr, values := range deprecatedServices {
			if gr == "main" {
				if _, ok := serviceDeclaredAlertmanagers["main"]; !ok {
					serviceDeclaredAlertmanagers["main"] = make([]alertmanagerServiceInfo, 0)
				}
				serviceDeclaredAlertmanagers["main"] = append(serviceDeclaredAlertmanagers["main"], values...)
			} else {
				serviceDeclaredAlertmanagers[gr] = values
			}
		}
	}

	input.Values.Set("prometheus.internal.alertmanagers", serviceDeclaredAlertmanagers)

	return nil
}

func handleDeperecatedAlertmanagerServices(input *go_hook.HookInput) map[string][]alertmanagerServiceInfo {
	snaps := input.Snapshots["alertmanager_services"]
	alertManagers := make(map[string][]alertmanagerServiceInfo)
	for _, svc := range snaps {
		alertManagerService := svc.(*alertmanagerService)

		if _, ok := alertManagers[alertManagerService.Prometheus]; !ok {
			alertManagers[alertManagerService.Prometheus] = make([]alertmanagerServiceInfo, 0)
		}
		alertManagers[alertManagerService.Prometheus] = append(alertManagers[alertManagerService.Prometheus], alertManagerService.Service)
	}

	return alertManagers
}

func parseServiceCR(am Alertmanager, snap []go_hook.FilterResult) (alertmanagerServiceInfo, error) {
	var value alertmanagerServiceInfo
	serviceName, ok, err := unstructured.NestedString(am.Spec, "external", "service", "name")
	if err != nil {
		return value, err
	}
	if !ok {
		return value, errors.New("service name required")
	}

	serviceNamespace, ok, err := unstructured.NestedString(am.Spec, "external", "service", "namespace")
	if err != nil {
		return value, err
	}
	if !ok {
		return value, errors.New("service namespace required")
	}

	pathPrefix, ok, err := unstructured.NestedString(am.Spec, "external", "service", "path")
	if err != nil || !ok {
		pathPrefix = "/"
	}

	for _, s := range snap {
		svc := s.(alertmanagerCRDService)
		if svc.Name == serviceName && svc.Namespace == serviceNamespace {
			value.Name = svc.Name
			value.Namespace = svc.Namespace
			value.Port = svc.Port
			value.PathPrefix = pathPrefix
			break
		}
	}

	return value, nil
}

func parseTargetCR(am Alertmanager) (alertmanagerValue, error) {
	var value alertmanagerValue

	address, ok, err := unstructured.NestedString(am.Spec, "external", "address")
	if err != nil || !ok {
		return value, errors.New("alertmanager address required")
	}

	parsedAddress, err := url.Parse(address)
	if err != nil {
		return value, err
	}

	ca, _, _ := unstructured.NestedString(am.Spec, "external", "tls", "ca")

	cert, _, _ := unstructured.NestedString(am.Spec, "external", "tls", "cert")

	key, _, _ := unstructured.NestedString(am.Spec, "external", "tls", "key")

	insecureSkipVerify, _, _ := unstructured.NestedBool(am.Spec, "external", "tls", "insecureSkipVerify")

	username, _, _ := unstructured.NestedString(am.Spec, "external", "auth", "basic", "username")

	password, _, _ := unstructured.NestedString(am.Spec, "external", "auth", "basic", "password")

	bearerToken, _, _ := unstructured.NestedString(am.Spec, "external", "auth", "bearerToken")

	value = alertmanagerValue{
		Name:   am.Name,
		Scheme: parsedAddress.Scheme,
		Target: parsedAddress.Host,
		Path:   parsedAddress.Path,
		BasicAuth: basicAuth{
			Username: username,
			Password: password,
		},
		BearerToken: bearerToken,
		TLSConfig: tlsConfig{
			Cert:               cert,
			Key:                key,
			CA:                 ca,
			InsecureSkipVerify: insecureSkipVerify,
		},
	}

	return value, nil
}

func applyDeprecatedAlertmanagerServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &corev1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, err
	}

	as := &alertmanagerService{}

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

type alertmanagerValue struct {
	Name        string    `json:"name" yaml:"name"`
	Scheme      string    `json:"scheme" yaml:"scheme"`
	Target      string    `json:"target" yaml:"target"`
	Path        string    `json:"path,omitempty" yaml:"path,omitempty"`
	BasicAuth   basicAuth `json:"basicAuth,omitempty" yaml:"basicAuth,omitempty"`
	BearerToken string    `json:"bearerToken,omitempty" yaml:"bearerToken,omitempty"`
	TLSConfig   tlsConfig `json:"tlsConfig,omitempty" yaml:"tlsConfig,omitempty"`
}

type basicAuth struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
}

type tlsConfig struct {
	CA                 string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Cert               string `json:"cert,omitempty" yaml:"cert,omitempty"`
	Key                string `json:"key,omitempty" yaml:"key,omitempty"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
}

type alertmanagerService struct {
	Prometheus string                  `json:"prometheus"`
	Service    alertmanagerServiceInfo `json:"service"`
}

type alertmanagerServiceInfo struct {
	Name       string      `json:"name"`
	Namespace  string      `json:"namespace"`
	PathPrefix string      `json:"pathPrefix"`
	Port       interface{} `json:"port"`
}
