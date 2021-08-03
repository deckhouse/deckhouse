/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func nameFromService(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	service := &v1.Service{}
	err := sdk.FromUnstructured(obj, service)
	if err != nil {
		return nil, err
	}

	if label, ok := service.Labels["prometheus-target"]; ok {
		return label, nil
	}

	if label, ok := service.Labels["prometheus.deckhouse.io/target"]; ok {
		return label, nil
	}

	return "", fmt.Errorf("possible bug, no desired label found")
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service-old",
			ApiVersion: "v1",
			Kind:       "Service",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "prometheus-target",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: nameFromService,
		},
		{
			Name:       "service",
			ApiVersion: "v1",
			Kind:       "Service",
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "prometheus.deckhouse.io/target",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
			FilterFunc: nameFromService,
		},
	},
}, discoverApps)

func discoverApps(input *go_hook.HookInput) error {
	const (
		enabledApplicationsSummaryPath = "monitoringApplications.internal.enabledApplicationsSummary"
		enabledApplicationsPath        = "monitoringApplications.enabledApplications"
	)

	enabledApplications := make(map[string]struct{})

	for _, app := range input.Snapshots["service-old"] {
		convertedApp := app.(string)
		enabledApplications[convertedApp] = struct{}{}
	}

	input.MetricsCollector.Set("d8_monitoring_applications_old_prometheus_target_total", float64(len(enabledApplications)), nil)

	for _, app := range input.Snapshots["service"] {
		convertedApp := app.(string)
		enabledApplications[convertedApp] = struct{}{}
	}

	appsFromConfig := input.Values.Get(enabledApplicationsPath).Array()
	for _, app := range appsFromConfig {
		enabledApplications[app.String()] = struct{}{}
	}

	result := make([]string, 0, len(enabledApplications))
	for app := range enabledApplications {
		result = append(result, app)
	}

	input.Values.Set(enabledApplicationsSummaryPath, result)
	return nil
}
