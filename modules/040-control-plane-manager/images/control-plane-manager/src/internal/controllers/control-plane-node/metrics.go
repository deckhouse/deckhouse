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

package controlplanenode

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

var controlPlaneNodeMaintenanceModeEnabled = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_control_plane_manager_maintenance_mode_enabled",
		Help: "Maintenance mode status for control-plane nodes.",
	},
	[]string{"node"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(controlPlaneNodeMaintenanceModeEnabled)
}

func syncMaintenanceModeMetrics(cpn *controlplanev1alpha1.ControlPlaneNode) {
	if cpn == nil {
		return
	}

	value := 0.0
	if isMaintenanceMode(cpn) {
		value = 1.0
	}

	controlPlaneNodeMaintenanceModeEnabled.WithLabelValues(cpn.Name).Set(value)
}

func deleteMaintenanceModeMetrics(node string) {
	if node == "" {
		return
	}
	controlPlaneNodeMaintenanceModeEnabled.DeleteLabelValues(node)
}
