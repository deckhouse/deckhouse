/*
Copyright 2026 Flant JSC

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

package waypointcontroller

import (
	"strings"

	networkv1alpha1 "waypoint-controller/pkg/apis/network.deckhouse.io/v1alpha1"
)

const (
	AppLabelKey               = "app"
	AppLabelValue             = "d8-waypoint"
	WaypointInstanceLabelKey  = "istio.deckhouse.io/waypoint-instance"
	WaypointComponentLabelKey = "istio.deckhouse.io/component"
	HeritageLabelKey          = "heritage"
	HeritageLabelValue        = "deckhouse"
	WaypointFinalizer         = "network.deckhouse.io/waypoint-instance-cleanup"
	ResourceNamePrefix        = "d8-waypoint-"
)

func resourceBaseName(instanceName string) string {
	return ResourceNamePrefix + instanceName
}

func instanceLabels(instance *networkv1alpha1.WaypointInstance) map[string]string {
	return map[string]string{
		AppLabelKey:              AppLabelValue,
		WaypointInstanceLabelKey: instance.Name,
		HeritageLabelKey:         HeritageLabelValue,
	}
}

func istioLabels(instance *networkv1alpha1.WaypointInstance, revision, networkName string) map[string]string {
	waypointFor := "All"
	if instance.Spec.WaypointFor != "" {
		waypointFor = instance.Spec.WaypointFor
	}

	return map[string]string{
		"gateway.istio.io/managed":  "istio.io-mesh-controller",
		"istio.io/rev":              revision,
		"istio.io/waypoint-for":     strings.ToLower(waypointFor),
		"topology.istio.io/network": networkName,
	}
}

func podTemplateLabels(instance *networkv1alpha1.WaypointInstance, revision, networkName string) map[string]string {
	labels := make(map[string]string)

	for k, v := range instanceLabels(instance) {
		labels[k] = v
	}

	for k, v := range istioLabels(instance, revision, networkName) {
		labels[k] = v
	}

	labels["istio.io/dataplane-mode"] = "none"
	labels["sidecar.istio.io/inject"] = "false"
	labels["service.istio.io/canonical-name"] = resourceBaseName(instance.Name)
	labels["service.istio.io/canonical-revision"] = "latest"
	labels["gateway.networking.k8s.io/gateway-name"] = resourceBaseName(instance.Name)

	return labels
}
