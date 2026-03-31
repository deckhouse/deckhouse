/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/pkg/log"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/metallb/alerting",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "services",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilterForAlerts,
		},
	},
}, checkServicesForDeprecatedAnnotations)

func applyServiceFilterForAlerts(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var service v1.Service

	err := sdk.FromUnstructured(obj, &service)
	if err != nil {
		return nil, err
	}

	if service.Spec.Type != v1.ServiceTypeLoadBalancer {
		return nil, nil
	}

	return ServiceInfoForAlert{
		Name:        service.Name,
		Namespace:   service.Namespace,
		Annotations: service.Annotations,
	}, nil
}

func checkServicesForDeprecatedAnnotations(_ context.Context, input *go_hook.HookInput) error {
	// Check Services' annotations
	input.MetricsCollector.Expire("D8MetallbNotSupportedServiceAnnotationsDetected")

	deprecatedAnnotations := [...]string{
		"metallb.universe.tf/address-pool",
		"metallb.universe.tf/loadBalancerIPs",
		"metallb.universe.tf/allow-shared-ip",
		"metallb.io/address-pool",
		"metallb.io/loadBalancerIPs",
		"metallb.io/allow-shared-ip",
	}

	serviceSnaps := input.Snapshots.Get("services")
	for service, err := range sdkobjectpatch.SnapshotIter[ServiceInfoForAlert](serviceSnaps) {
		if err != nil {
			input.Logger.Warn("iterate over services", log.Err(err))
			continue
		}

		for _, annotation := range deprecatedAnnotations {
			if _, ok := service.Annotations[annotation]; ok {
				input.MetricsCollector.Set("d8_metallb_not_supported_service_annotations_detected", 1,
					map[string]string{
						"name":       service.Name,
						"namespace":  service.Namespace,
						"annotation": annotation,
					}, metrics.WithGroup("D8MetallbNotSupportedServiceAnnotationsDetected"))
			}
		}
	}

	return nil
}
