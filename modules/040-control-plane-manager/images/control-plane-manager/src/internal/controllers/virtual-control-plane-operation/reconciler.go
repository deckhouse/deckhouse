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
	"fmt"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = (*reconciler)(nil)

type reconciler struct {
	client            client.Client
	operationExecutor operations.OperationExecutor
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	operation, err := r.getOperation(ctx, req.NamespacedName)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get controlPlaneOperation: %w", err)
	}

	if res, err := r.reconcileInitialConditions(ctx, operation); err != nil || !res.IsZero() {
		return res, err
	}

	if !operation.Spec.Approved || operation.IsTerminal() {
		return reconcile.Result{}, nil
	}

	if err := r.reconcileStartedAt(ctx, operation); err != nil {
		return reconcile.Result{}, fmt.Errorf("stamp startedAt annotation: %w", err)
	}

	return r.reconcileOperation(ctx, operation)
}

// reconcileStartedAt (In-place reconciliation on metadata): stamp the start annotation once.
func (r *reconciler) reconcileStartedAt(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) error {
	if operation.Annotations[constants.OperationStartedAtAnnotationKey] != "" {
		return nil
	}

	base := operation.DeepCopy()
	if operation.Annotations == nil {
		operation.Annotations = make(map[string]string, 1)
	}

	operation.Annotations[constants.OperationStartedAtAnnotationKey] = time.Now().UTC().Format(time.RFC3339Nano)

	return r.patchOperation(ctx, operation, base)
}

func (r *reconciler) reconcileOperation(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (reconcile.Result, error) {
	if needed, reason := r.operationExecutor.NeedsExecution(ctx, operation); !needed {
		return r.reconcileAbandonedOperation(ctx, operation, reason)
	}

	result := r.operationExecutor.Execute(ctx, operation)
	switch result.Outcome {
	case operations.OperationFailed:
		return r.reconcileFailedOperation(ctx, operation, result)
	case operations.OperationInProgress:
		return r.reconcileInProgressOperation(ctx, operation, result)
	default:
		return r.reconcileCompletedOperation(ctx, operation, result)
	}
}

func (r *reconciler) reconcileAbandonedOperation(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation, reason string) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyAbandonedOperation(operation, reason)
	return reconcile.Result{}, r.patchOperationStatus(ctx, operation, base)
}

func applyAbandonedOperation(operation *controlplanev1alpha1.ControlPlaneOperation, message string) {
	//markCurrentInProgressStep(op, controlplanev1alpha1.CPOReasonStepAbandoned, "") возможно нужно будет удалить
	setCondition(
		operation,
		controlplanev1alpha1.CPOConditionCompleted,
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonOperationAbandoned,
		message,
	)
}

func (r *reconciler) reconcileFailedOperation(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation, result operations.OperationResult) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyStepResults(operation, result.StepResults)
	applyOperationFailed(operation, result.Message)

	if err := r.patchOperationStatus(ctx, operation, base); err != nil {
		// TODO: log.FromContext(ctx).Error()
	}

	return reconcile.Result{}, result.Error
}

func applyOperationFailed(operation *controlplanev1alpha1.ControlPlaneOperation, message string) {
	setCondition(
		operation,
		controlplanev1alpha1.CPOConditionCompleted,
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonOperationFailed,
		message,
	)
}

func (r *reconciler) reconcileInProgressOperation(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation, result operations.OperationResult) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyStepResults(operation, result.StepResults)
	applyOperationInProgress(operation, result.Message)

	if err := r.patchOperationStatus(ctx, operation, base); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: result.RequeueAfter}, nil
}

func applyOperationInProgress(operation *controlplanev1alpha1.ControlPlaneOperation, message string) {
	setCondition(
		operation,
		controlplanev1alpha1.CPOConditionCompleted,
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonOperationInProgress,
		message,
	)
}

func (r *reconciler) reconcileCompletedOperation(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation, result operations.OperationResult) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyStepResults(operation, result.StepResults)
	applyOperationCompleted(operation)

	return reconcile.Result{}, r.patchOperationStatus(ctx, operation, base)
}

func applyOperationCompleted(operation *controlplanev1alpha1.ControlPlaneOperation) {
	setCondition(
		operation,
		controlplanev1alpha1.CPOConditionCompleted,
		metav1.ConditionTrue,
		controlplanev1alpha1.CPOReasonOperationCompleted,
		"operation completed",
	)
}

func applyStepResults(operation *controlplanev1alpha1.ControlPlaneOperation, results []operations.StepResult) {
	for _, result := range results {
		switch result.Status {
		case operations.StepFailed:
			applyStepFailed(operation, result.Name, result.Error.Error())
		case operations.StepProgressing:
			applyStepInProgress(operation, result.Name, result.Message)
		case operations.StepCompleted:
			applyStepCompleted(operation, result.Name, result.Message)
		}
	}
}

func applyStepFailed(operation *controlplanev1alpha1.ControlPlaneOperation, name controlplanev1alpha1.StepName, errorMessage string) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonStepFailed,
		errorMessage,
	)
}

func applyStepInProgress(operation *controlplanev1alpha1.ControlPlaneOperation, name controlplanev1alpha1.StepName, message string) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonStepInProgress,
		message,
	)
}

func applyStepCompleted(operation *controlplanev1alpha1.ControlPlaneOperation, name controlplanev1alpha1.StepName, message string) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionTrue,
		controlplanev1alpha1.CPOReasonStepCompleted,
		message,
	)
}

func (r *reconciler) getOperation(ctx context.Context, namespacedName types.NamespacedName) (*controlplanev1alpha1.ControlPlaneOperation, error) {
	operation := &controlplanev1alpha1.ControlPlaneOperation{}
	err := r.client.Get(ctx, namespacedName, operation)
	return operation, err
}

func (r *reconciler) patchOperation(ctx context.Context, operation, base *controlplanev1alpha1.ControlPlaneOperation) error {
	return r.client.Patch(ctx, operation, client.MergeFrom(base))
}

func (r *reconciler) patchOperationStatus(ctx context.Context, operation, base *controlplanev1alpha1.ControlPlaneOperation) error {
	return r.client.Status().Patch(ctx, operation, client.MergeFrom(base))
}
