/*
Copyright 2023 Flant JSC

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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// This hook scans ServiceMonitors in a cluster and find deprecated relabelings based on the `__meta_kubernetes_endpoints_` labels
// We fire the alert and ask user to migrate to a new labels
// TODO: This hook can be deleted in Deckhouse release 1.60
//  with `PrometheusServiceMonitorDeprecated` from modules/300-prometheus/monitoring/prometheus-rules/deprecation.yaml
//  and prometheus-operator patch modules/200-operator-prometheus/images/prometheus-operator/patches/002_endpointslices_fallback.patch

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/servicemonitors",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "servicemonitors",
			ApiVersion: "monitoring.coreos.com/v1",
			Kind:       "ServiceMonitor",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"prometheus": "main",
				},
			},
			FilterFunc: filterServiceMonitor,
		},
	},
}, serviceMonitorHandler)

func filterServiceMonitor(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var sm serviceMonitor
	err := sdk.FromUnstructured(obj, &sm)
	if err != nil {
		return nil, err
	}

	return sm, nil
}

func serviceMonitorHandler(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_servicemonitors")

	snaps := input.Snapshots.Get("servicemonitors")

	for serviceMon, err := range sdkobjectpatch.SnapshotIter[serviceMonitor](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'servicemonitors' snapshots: %w", err)
		}

	serviceMonitorLoop:
		for _, endpoint := range serviceMon.Spec.Endpoints {
			for _, relabel := range endpoint.Relabelings {
				tmpMap := make(map[string]struct{}, len(relabel.SourceLabels))
				for _, sourceLabel := range relabel.SourceLabels {
					if strings.HasPrefix(sourceLabel, "__meta_kubernetes_endpoint") {
						sourceLabel = mutateLabelOrAnnotationSource(sourceLabel)
						tmpMap[sourceLabel] = struct{}{}
					}
				}

				if len(tmpMap) > 0 {
					// check if sourceLabel has `_endpoint` label but does not have `_endpoint_slice` label
					for k, v := range endpointsMap {
						_, ok1 := tmpMap[k]
						_, ok2 := tmpMap[v]
						if ok1 && !ok2 {
							input.MetricsCollector.Set("d8_prometheus_deprecated_servicemonitor", 1, map[string]string{"name": serviceMon.Metadata.Name, "namespace": serviceMon.Metadata.Namespace}, metrics.WithGroup("d8_servicemonitors"))
							break serviceMonitorLoop
						}
					}
				}
			}
		}
	}
	return nil
}

func mutateLabelOrAnnotationSource(label string) string {
	if strings.HasPrefix(label, "__meta_kubernetes_endpoints_label_") {
		return "__meta_kubernetes_endpoints_label_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpoints_labelpresent_") {
		return "__meta_kubernetes_endpoints_labelpresent_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpoints_annotation_") {
		return "__meta_kubernetes_endpoints_annotation_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpoints_annotationpresent_") {
		return "__meta_kubernetes_endpoints_annotationpresent_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpointslice_label_") {
		return "__meta_kubernetes_endpointslice_label_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpointslice_labelpresent_") {
		return "__meta_kubernetes_endpointslice_labelpresent_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpointslice_annotation_") {
		return "__meta_kubernetes_endpointslice_annotation_"
	}
	if strings.HasPrefix(label, "__meta_kubernetes_endpointslice_annotationpresent_") {
		return "__meta_kubernetes_endpointslice_annotationpresent_"
	}

	return label
}

// cutted ServiceMonitor
type serviceMonitor struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Endpoints []struct {
			Relabelings []struct {
				SourceLabels []string `json:"sourceLabels"`
			} `json:"relabelings"`
		} `json:"endpoints"`
	} `json:"spec"`
}

var (
	endpointsMap = map[string]string{
		"__meta_kubernetes_endpoints_name":               "__meta_kubernetes_endpointslice_name",
		"__meta_kubernetes_endpoint_node_name":           "__meta_kubernetes_endpointslice_endpoint_topology_kubernetes_io_hostname",
		"__meta_kubernetes_endpoint_ready":               "__meta_kubernetes_endpointslice_endpoint_conditions_ready",
		"__meta_kubernetes_endpoint_port_name":           "__meta_kubernetes_endpointslice_port_name",
		"__meta_kubernetes_endpoint_port_protocol":       "__meta_kubernetes_endpointslice_port_protocol",
		"__meta_kubernetes_endpoint_address_target_kind": "__meta_kubernetes_endpointslice_address_target_kind",
		"__meta_kubernetes_endpoint_address_target_name": "__meta_kubernetes_endpointslice_address_target_name",

		// prefix labels
		"__meta_kubernetes_endpoints_label_":             "__meta_kubernetes_endpointslice_label_",
		"__meta_kubernetes_endpoints_labelpresent_":      "__meta_kubernetes_endpointslice_labelpresent_",
		"__meta_kubernetes_endpoints_annotation_":        "__meta_kubernetes_endpointslice_annotation_",
		"__meta_kubernetes_endpoints_annotationpresent_": "__meta_kubernetes_endpointslice_annotationpresent_",
	}
)
