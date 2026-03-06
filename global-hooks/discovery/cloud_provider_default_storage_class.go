// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Order 25: run after module discovery hooks (typically Order 20)
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 25},
}, discoverCloudProviderDefaultStorageClass)

func discoverCloudProviderDefaultStorageClass(_ context.Context, input *go_hook.HookInput) error {
	const (
		discoveryPath = "global.discovery.cloudProviderDefaultStorageClass"
		metricName    = "d8_cloud_provider_dvp_default_storage_class_drifted"
		metricGroup   = "cloud_provider_dvp_default_storage_class"
	)

	// Try to get default storage class from DVP cloud provider discovery data
	defaultSC := discoverDefaultStorageClassFromDVP(input)

	if defaultSC != "" {
		input.Values.Set(discoveryPath, defaultSC)
		input.Logger.Info("Discovered default storage class from DVP cloud provider", slog.String("storage_class", defaultSC))

		// Detect drift
		detectAndReportDrift(input, defaultSC, metricName, metricGroup)
		return nil
	}

	// No default storage class found from cloud provider
	input.Logger.Info("No default storage class found in parent DVP cluster")
	input.Values.Remove(discoveryPath)
	input.MetricsCollector.Expire(metricName)

	return nil
}

// discoverDefaultStorageClassFromDVP extracts default StorageClass name from DVP provider discovery data
func discoverDefaultStorageClassFromDVP(input *go_hook.HookInput) string {
	dvpDiscoveryData, exists := input.Values.GetOk("cloudProviderDvp.internal.providerDiscoveryData.storageClasses")
	if !exists || !dvpDiscoveryData.Exists() {
		return ""
	}

	for _, sc := range dvpDiscoveryData.Array() {
		if sc.Get("isDefault").Bool() {
			return sc.Get("name").String()
		}
	}

	return ""
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
