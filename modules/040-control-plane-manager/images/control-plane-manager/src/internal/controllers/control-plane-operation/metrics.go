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
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// operationInprogressStart stores the unix timestamp when the current in-progress operation started.
// Prometheus alert uses time() - this_value to compute real-time duration without staleness.
var operationInprogressStart = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "d8_control_plane_manager_operation_inprogress_start_seconds",
		Help: "Unix timestamp when the current in-progress operation started (per node/component/operation).",
	},
	[]string{"node", "component", "operation"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(operationInprogressStart)
}

func syncOperationExecutionMetrics(op *controlplanev1alpha1.ControlPlaneOperation) {
	if op == nil {
		return
	}

	nodeLabel := op.Labels[constants.ControlPlaneNodeNameLabelKey]
	componentLabel := string(op.Spec.Component)
	operationLabel := op.Name

	cond := op.GetCondition(controlplanev1alpha1.CPOConditionCompleted)
	if cond != nil &&
		cond.Reason == controlplanev1alpha1.CPOReasonOperationInProgress &&
		!cond.LastTransitionTime.IsZero() {
		operationInprogressStart.WithLabelValues(nodeLabel, componentLabel, operationLabel).Set(float64(cond.LastTransitionTime.Unix()))
		return
	}

	operationInprogressStart.DeleteLabelValues(nodeLabel, componentLabel, operationLabel)
}
