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

package virtualcontrolplaneoperation

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *reconciler) reconcileInitialConditions(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation) (reconcile.Result, error) {
	if op.IsTerminal() {
		return reconcile.Result{}, nil
	}

	base := op.DeepCopy()
	if !ensureInitialConditions(op) {
		return reconcile.Result{}, nil
	}

	if err := r.patchOperationStatus(ctx, op, base); err != nil {
		return reconcile.Result{}, fmt.Errorf("seed initial conditions: %w", err)
	}

	return reconcile.Result{}, nil
}

func ensureInitialConditions(operation *controlplanev1alpha1.ControlPlaneOperation) bool {
	changed := false

	if meta.FindStatusCondition(operation.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted) == nil {
		setCondition(operation, controlplanev1alpha1.CPOConditionCompleted, metav1.ConditionUnknown, controlplanev1alpha1.CPOReasonOperationPending, "")
		changed = true
	}

	for _, name := range operation.Spec.Steps {
		condType := controlplanev1alpha1.StepConditionType(name)
		if meta.FindStatusCondition(operation.Status.Conditions, condType) == nil {
			setCondition(operation, condType, metav1.ConditionFalse, controlplanev1alpha1.CPOReasonStepUnknown, "")
			changed = true
		}
	}

	return changed
}

func setCondition(op *controlplanev1alpha1.ControlPlaneOperation, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&op.Status.Conditions, metav1.Condition{
		Type:    condType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
