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
	"errors"
	"fmt"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

const (
	maintenanceModeEnabledMetricName = "d8_control_plane_manager_maintenance_mode_enabled"
	maintenanceModeEnabledHelp       = "Maintenance mode status for control-plane nodes."
)

type metrics struct {
	maintenanceMode *collectors.ConstGaugeCollector
}

func newMetrics(storage metricsstorage.Storage) (*metrics, error) {
	if storage == nil {
		return nil, errors.New("metric storage is nil")
	}

	maintenanceMode, err := storage.RegisterGauge(
		maintenanceModeEnabledMetricName,
		[]string{"node"},
		options.WithHelp(maintenanceModeEnabledHelp),
	)
	if err != nil {
		return nil, fmt.Errorf("register maintenance mode metric: %w", err)
	}

	return &metrics{
		maintenanceMode: maintenanceMode,
	}, nil
}

func maintenanceModeGroup(node string) string {
	return "cpn/" + node
}

func (m *metrics) syncMaintenanceModeMetrics(cpn *controlplanev1alpha1.ControlPlaneNode) {
	if m == nil || cpn == nil {
		return
	}

	value := 0.0
	if isMaintenanceMode(cpn) {
		value = 1.0
	}

	m.maintenanceMode.Set(
		value,
		map[string]string{
			"node": cpn.Name,
		},
		collectors.WithGroup(maintenanceModeGroup(cpn.Name)),
	)
}

func (m *metrics) deleteMaintenanceModeMetrics(node string) {
	if m == nil || node == "" {
		return
	}

	m.maintenanceMode.ExpireGroupMetrics(maintenanceModeGroup(node))
}
