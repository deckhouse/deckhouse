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

package controlplaneoperation

import (
	"errors"
	"fmt"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/collectors"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Prometheus alert uses time() - this_value to compute real-time duration without staleness.
const (
	operationInProgressMetricName = "d8_control_plane_manager_operation_inprogress_start_seconds"
	operationInProgressMetricHelp = "Unix timestamp when the current in-progress operation started (per node/component/operation)."
)

type metrics struct {
	operationInProgress *collectors.ConstGaugeCollector
}

func newMetrics(storage metricsstorage.Storage) (*metrics, error) {
	if storage == nil {
		return nil, errors.New("metric storage is nil")
	}

	operationInProgress, err := storage.RegisterGauge(
		operationInProgressMetricName,
		[]string{"node", "component", "operation"},
		options.WithHelp(operationInProgressMetricHelp),
	)
	if err != nil {
		return nil, fmt.Errorf("register operation in progress metric: %w", err)
	}

	return &metrics{
		operationInProgress: operationInProgress,
	}, nil
}

func operationExecutionGroup(operation string) string {
	return "cpo/" + operation
}

func (m *metrics) syncOperationExecutionMetrics(op *controlplanev1alpha1.ControlPlaneOperation) {
	if m == nil || op == nil {
		return
	}

	nodeLabel := op.Labels[constants.ControlPlaneNodeNameLabelKey]
	componentLabel := string(op.Spec.Component)
	operationLabel := op.Name

	cond := op.GetCondition(controlplanev1alpha1.CPOConditionCompleted)
	if cond != nil && cond.Reason == controlplanev1alpha1.CPOReasonOperationInProgress && !cond.LastTransitionTime.IsZero() {
		m.operationInProgress.Set(
			float64(cond.LastTransitionTime.Unix()),
			map[string]string{
				"node":      nodeLabel,
				"component": componentLabel,
				"operation": operationLabel,
			},
			collectors.WithGroup(operationExecutionGroup(operationLabel)),
		)
		return
	}

	m.operationInProgress.ExpireGroupMetrics(operationExecutionGroup(operationLabel))
}

func (m *metrics) deleteOperationExecutionMetrics(operation string) {
	if m == nil || operation == "" {
		return
	}

	m.operationInProgress.ExpireGroupMetrics(operationExecutionGroup(operation))
}
