package hooks

import (
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
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	PathPrefix string `json:"pathPrefix"`
	Port       int32  `json:"port"`
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

	for _, port := range service.Spec.Ports {
		as.Service.Port = port.Port
		break
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
