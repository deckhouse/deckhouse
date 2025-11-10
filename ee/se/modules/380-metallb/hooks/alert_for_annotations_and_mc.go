/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
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
		{
			Name:       "module_config",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"metallb"},
			},
			FilterFunc: applyModuleConfigFilterForAlerts,
		},
	},
}, checkServicesForDeprecatedAnnotations)

func applyModuleConfigFilterForAlerts(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	mc := &ModuleConfig{}
	err := sdk.FromUnstructured(obj, mc)
	if err != nil {
		return nil, fmt.Errorf("cannot convert Metallb ModuleConfig: %v", err)
	}
	return mc, nil
}

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
	// Check ModuleConfig version and pools
	input.MetricsCollector.Expire("D8MetallbUpdateMCVersionRequired")
	input.MetricsCollector.Expire("D8MetallbObsoleteLayer2PoolsAreUsed")

	mcSnaps := input.Snapshots.Get("module_config")
	if len(mcSnaps) != 1 {
		return nil
	}

	mc := new(ModuleConfig)

	err := mcSnaps[0].UnmarshalTo(mc)
	if err != nil {
		return fmt.Errorf("cannot unmarshal ModuleConfig: %w", err)
	}

	if mc.Spec.Version >= 2 {
		for _, pool := range mc.Spec.Settings.AddressPools {
			if pool.Protocol == "layer2" {
				input.MetricsCollector.Set("d8_metallb_obsolete_layer2_pools_are_used", 1,
					map[string]string{"name": pool.Name},
					metrics.WithGroup("D8MetallbObsoleteLayer2PoolsAreUsed"))
			}
		}

		return nil
	}

	for _, pool := range mc.Spec.Settings.AddressPools {
		if pool.Protocol == "bgp" {
			return nil
		}
	}
	input.MetricsCollector.Set("d8_metallb_update_mc_version_required", 1,
		map[string]string{}, metrics.WithGroup("D8MetallbUpdateMCVersionRequired"))

	// Check Services' annotations
	input.MetricsCollector.Expire("D8MetallbNotSupportedServiceAnnotationsDetected")

	var deprecatedAnnotations = [...]string{
		"metallb.io/ip-allocated-from-pool",
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
