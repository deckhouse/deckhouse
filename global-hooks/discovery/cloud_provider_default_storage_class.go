/*
Copyright 2025 Flant JSC

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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:       "/modules/cloud-provider-dvp",
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "check_default_storage_class",
			Crontab: "*/5 * * * *", // Every 5 minutes
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_discovery_data",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cloud-provider-discovery-data"},
			},
			FilterFunc: applyCloudProviderSecretFilter,
		},
	},
}, handleCloudProviderDefaultStorageClass)

func applyCloudProviderSecretFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return &corev1.Secret{}, nil
}

func handleCloudProviderDefaultStorageClass(_ context.Context, input *go_hook.HookInput) error {
	const (
		discoveryPath = "global.discovery.cloudProviderDefaultStorageClass"
		metricName    = "d8_cloud_provider_dvp_default_storage_class_drifted"
		metricGroup   = "cloud_provider_dvp_default_storage_class"
	)

	// Read default storage class from cloud-provider-dvp module internal values
	defaultSC := input.Values.Get("cloudProviderDvp.internal.defaultStorageClass").String()

	if defaultSC != "" {
		input.Values.Set(discoveryPath, defaultSC)
		input.Logger.Info("Set cloud provider default storage class to global values", slog.String("storage_class", defaultSC))

		// Detect drift
		detectAndReportDrift(input, defaultSC, metricName, metricGroup)
	} else {
		input.Logger.Info("No default storage class found from cloud provider")
		input.Values.Remove(discoveryPath)
		input.MetricsCollector.Expire(metricName)
	}

	return nil
}

// detectAndReportDrift compares expected default SC with actual and reports metric if drifted
func detectAndReportDrift(input *go_hook.HookInput, expectedSC, metricName, metricGroup string) {
	actualDefaultSC := input.Values.Get("global.discovery.defaultStorageClass").String()

	// No actual default SC in cluster yet - no drift
	if actualDefaultSC == "" {
		input.MetricsCollector.Expire(metricName)
		return
	}

	// Check for drift
	if actualDefaultSC != expectedSC {
		input.Logger.Warn("Default storage class drift detected",
			slog.String("expected", expectedSC),
			slog.String("actual", actualDefaultSC),
		)
		input.MetricsCollector.Set(
			metricName,
			1.0,
			map[string]string{
				"expected": expectedSC,
				"actual":   actualDefaultSC,
			},
			metrics.WithGroup(metricGroup),
		)
	} else {
		// No drift - expire the metric
		input.MetricsCollector.Expire(metricName)
	}
}
