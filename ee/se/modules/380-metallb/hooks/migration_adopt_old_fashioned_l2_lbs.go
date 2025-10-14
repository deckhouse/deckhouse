/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license.
See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Depends on 'migration-auto-generating-l2-mlbc.go' hook, we need the ipAddressPoolToMLBCMap here
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/metallb/discovery",
}, dependency.WithExternalDependencies(discoveryServicesForMigrate))

func discoveryServicesForMigrate(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	// Get ModuleConfig and check requirements
	unstructuredMC, err := k8sClient.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "moduleconfigs",
	}).Get(context.TODO(), "metallb", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error to read ModuleConfig: %w", err)
	}

	var moduleConfig ModuleConfig
	err = sdk.FromUnstructured(unstructuredMC, &moduleConfig)
	if err != nil {
		return err
	}
	if moduleConfig.Spec.Version >= 2 {
		input.Values.Set("metallb.internal.migrationOfOldFashionedLBsAdoptionComplete", true)
		input.Logger.Info("processing skipped", "ModuleConfig version", moduleConfig.Spec.Version)
		return nil
	}
	for _, pool := range moduleConfig.Spec.Settings.AddressPools {
		if pool.Protocol == "bgp" {
			input.Values.Set("metallb.internal.migrationOfOldFashionedLBsAdoptionComplete", true)
			return nil
		}
	}

	// Get list of Services
	serviceList, err := k8sClient.CoreV1().Services("").List(
		context.TODO(),
		metav1.ListOptions{},
	)
	if err != nil {
		return nil
	}

	for _, service := range serviceList.Items {
		// Is it a Loadbalancer?
		if service.Spec.Type != "LoadBalancer" {
			continue
		}
		if len(service.Status.LoadBalancer.Ingress) == 0 {
			continue
		}

		// Has MLBC status?
		statusExists := false
		for _, condition := range service.Status.Conditions {
			if condition.Type == "network.deckhouse.io/load-balancer-class" {
				statusExists = true
				break
			}
		}
		if statusExists {
			continue
		}

		// Has the annotations?
		if _, ok := service.Annotations["network.deckhouse.io/load-balancer-ips"]; ok {
			continue
		}
		if _, ok := service.Annotations["network.deckhouse.io/metal-load-balancer-class"]; ok {
			continue
		}

		// Patch the service
		var sliceIPs []string
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			sliceIPs = append(sliceIPs, ingress.IP)
		}
		stringIPs := strings.Join(sliceIPs, ",")

		annotations := map[string]any{
			"network.deckhouse.io/load-balancer-ips": stringIPs,
		}
		annotationKeys := []string{
			"metallb.universe.tf/ip-allocated-from-pool",
			"metallb.universe.tf/address-pool",
		}
		for _, key := range annotationKeys {
			if poolName, ok := service.Annotations[key]; ok {
				if poolsMap, ok := input.Values.GetOk("metallb.internal.ipAddressPoolToMLBCMap"); ok {
					if mlbcName, ok := poolsMap.Map()[poolName]; ok {
						annotations["network.deckhouse.io/metal-load-balancer-class"] = mlbcName.String()
					}
				}
			}
		}

		patch := map[string]any{
			"metadata": map[string]any{
				"annotations": annotations,
				"finalizers":  []string{},
			},
		}

		data, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("error to marshal patch-object: %w", err)
		}

		_, err = k8sClient.CoreV1().Services(service.Namespace).Patch(
			context.TODO(),
			service.Name,
			types.MergePatchType,
			data,
			metav1.PatchOptions{},
		)
		if err != nil {
			return fmt.Errorf("error to apply patch to Service %s: %w", service.Name, err)
		}
		input.Logger.Info("annotations added", "Service", service.Name)
	}
	input.Values.Set("metallb.internal.migrationOfOldFashionedLBsAdoptionComplete", true)
	return nil
}
