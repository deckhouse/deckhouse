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
	"net/url"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/k8s"
	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "alertmanager_crds",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "CustomAlertmanager",
			FilterFunc: applyAlertmanagerCRDFilter,
		},
		{
			// deprecated way to set alertmanagers - through the labeled service
			Name:       "alertmanager_deprecated_services",
			ApiVersion: "v1",
			Kind:       "Service",
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "prometheus.deckhouse.io/alertmanager",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"main"},
				},
			}},
			FilterFunc: applyDeprecatedAlertmanagerServiceFilter,
		},
	},
}, dependency.WithExternalDependencies(crdAndServicesAlertmanagerHandler))

func applyDeprecatedAlertmanagerServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &corev1.Service{}
	as := &alertmanagerService{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, fmt.Errorf("cannot convert to service: %v", err)
	}

	as.Namespace = service.ObjectMeta.Namespace
	as.Name = service.ObjectMeta.Name

	switch {
	case len(service.Spec.Ports[0].Name) != 0:
		as.Port = service.Spec.Ports[0].Name
	case service.Spec.Ports[0].Port != 0:
		as.Port = service.Spec.Ports[0].Port
	default:
		return nil, fmt.Errorf("can't find Name or Port in the first port of a Service %#+v", as)
	}

	as.PathPrefix = "/"
	if prefix, ok := service.ObjectMeta.Annotations["prometheus.deckhouse.io/alertmanager-path-prefix"]; ok {
		as.PathPrefix = prefix
	}

	return as, nil
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

func crdAndServicesAlertmanagerHandler(input *go_hook.HookInput, dc dependency.Container) error {
	k8, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("can't init Kubernetes client: %v", err)
	}

	snap := input.Snapshots["alertmanager_crds"]

	addressDeclaredAlertmanagers := make([]alertmanagerAddress, 0, len(snap))
	serviceDeclaredAlertmanagers := make([]alertmanagerService, 0, len(snap))
	internalDeclaredAlertmanagers := make([]alertmanagerInternal, 0, len(snap))

	for _, s := range snap {
		am := s.(Alertmanager)

		// set observed status
		input.PatchCollector.Filter(set_cr_statuses.SetObservedStatus(s, applyAlertmanagerCRDFilter), "deckhouse.io/v1alpha1", "customalertmanager", "", am.Name, object_patch.WithSubresource("/status"), object_patch.IgnoreHookError())

		// External AlertManagers by service or address
		if _, ok, _ := unstructured.NestedMap(am.Spec, "external"); ok {
			address, _, _ := unstructured.NestedString(am.Spec, "external", "address")
			if address != "" {
				// parse static_sd_config with direct target
				value, err := parseTargetCR(am)
				if err != nil {
					return err
				}
				addressDeclaredAlertmanagers = append(addressDeclaredAlertmanagers, value)
			} else {
				// parse service
				old, err := parseServiceCR(am, k8)
				if err != nil {
					return err
				}
				serviceDeclaredAlertmanagers = append(serviceDeclaredAlertmanagers, old)
			}
		}
		// Internal AlertManager
		if _, ok, _ := unstructured.NestedMap(am.Spec, "internal"); ok {
			value, err := parseInternalCR(am)
			if err != nil {
				return err
			}
			internalDeclaredAlertmanagers = append(internalDeclaredAlertmanagers, value)
		}
	}

	// External Alertmanagers by deprecated labeled services
	deprecatedServiceDeclaredAlertmanagers := handleDeprecatedAlertmanagerServices(input)
	serviceDeclaredAlertmanagers = append(serviceDeclaredAlertmanagers, deprecatedServiceDeclaredAlertmanagers...)

	input.Values.Set("prometheus.internal.alertmanagers.byAddress", addressDeclaredAlertmanagers)
	input.Values.Set("prometheus.internal.alertmanagers.byService", serviceDeclaredAlertmanagers)
	input.Values.Set("prometheus.internal.alertmanagers.internal", internalDeclaredAlertmanagers)

	return nil
}

func handleDeprecatedAlertmanagerServices(input *go_hook.HookInput) []alertmanagerService {
	snaps := input.Snapshots["alertmanager_deprecated_services"]
	alertManagers := make([]alertmanagerService, 0, len(snaps))
	for _, svc := range snaps {
		alertManagerService := svc.(*alertmanagerService)
		alertManagers = append(alertManagers, *alertManagerService)
	}

	return alertManagers
}

func parseServiceCR(am Alertmanager, k8 k8s.Client) (alertmanagerService, error) {
	var value alertmanagerService
	serviceName, ok, err := unstructured.NestedString(am.Spec, "external", "service", "name")
	if err != nil {
		return value, err
	}
	if !ok {
		return value, fmt.Errorf("service name required: %v", am.Spec)
	}

	serviceNamespace, ok, err := unstructured.NestedString(am.Spec, "external", "service", "namespace")
	if err != nil {
		return value, err
	}
	if !ok {
		return value, fmt.Errorf("service namespace required: %v", am.Spec)
	}

	pathPrefix, ok, err := unstructured.NestedString(am.Spec, "external", "service", "path")
	if err != nil || !ok {
		pathPrefix = "/"
	}

	svc, err := k8.CoreV1().Services(serviceNamespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if err != nil {
		return value, err
	}

	value.ResourceName = am.Name
	value.Name = svc.Name
	value.Namespace = svc.Namespace
	if len(svc.Spec.Ports) > 0 {
		switch {
		case len(svc.Spec.Ports[0].Name) != 0:
			value.Port = svc.Spec.Ports[0].Name
		case svc.Spec.Ports[0].Port != 0:
			value.Port = svc.Spec.Ports[0].Port
		default:
			return value, fmt.Errorf("can't find Name or Port in the first port of a Service %#+v", svc)
		}
	}
	value.PathPrefix = pathPrefix

	return value, nil
}

func parseTargetCR(am Alertmanager) (alertmanagerAddress, error) {
	var value alertmanagerAddress

	address, ok, err := unstructured.NestedString(am.Spec, "external", "address")
	if err != nil || !ok {
		return value, fmt.Errorf("alertmanager address required: %v", am.Spec)
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

	value = alertmanagerAddress{
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

func parseInternalCR(am Alertmanager) (alertmanagerInternal, error) {
	value, ok, err := unstructured.NestedMap(am.Spec, "internal")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("internal spec field required: %v", am.Spec)
	}

	value["name"] = am.Name
	return value, nil
}

type alertmanagerAddress struct {
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
	ResourceName string      `json:"resourceName"`
	Name         string      `json:"name"`
	Namespace    string      `json:"namespace"`
	PathPrefix   string      `json:"pathPrefix"`
	Port         interface{} `json:"port"`
}

type Alertmanager struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

type alertmanagerInternal map[string]interface{}
