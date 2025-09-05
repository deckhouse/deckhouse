/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/go_lib/set"
)

func getAllowedApplications() (set.Set, error) {
	applicationPaths := "/deckhouse/modules/340-monitoring-applications/applications/*"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		applicationPaths = "/deckhouse/ee/fe/modules/340-monitoring-applications/applications/*"
	}

	res, err := filepath.Glob(applicationPaths)
	if err != nil {
		return nil, err
	}

	apps := set.New()
	for _, match := range res {
		apps.Add(filepath.Base(match))
	}

	return apps, nil
}

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

func discoverApps(_ context.Context, input *go_hook.HookInput) error {
	const (
		allowedApplicationsPath        = "monitoringApplications.internal.allowedApplications"
		enabledApplicationsSummaryPath = "monitoringApplications.internal.enabledApplicationsSummary"
		enabledApplicationsPath        = "monitoringApplications.enabledApplications"
	)

	allowedApplications, err := getAllowedApplications()
	if err != nil {
		return err
	}
	input.Values.Set(allowedApplicationsPath, allowedApplications.Slice())

	enabledApps := set.NewFromSnapshot(input.Snapshots.Get("service-old"))

	input.MetricsCollector.Set("d8_monitoring_applications_old_prometheus_target_total", float64(len(enabledApps)), nil)

	enabledApps.
		AddSet(set.NewFromSnapshot(input.Snapshots.Get("service"))).
		AddSet(set.NewFromValues(input.Values, enabledApplicationsPath))

	// Add dashboards for default applications to the cluster
	if module.IsEnabled("prometheus", input) {
		enabledApps.Add("prometheus")
	}

	enabledApps = enabledApps.Intersection(allowedApplications)

	input.Values.Set(enabledApplicationsSummaryPath, enabledApps.Slice())
	return nil
}
