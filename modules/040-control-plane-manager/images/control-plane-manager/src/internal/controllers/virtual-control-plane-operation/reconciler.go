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
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

func (r *reconciler) reconcileOperation(
	ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation,
) (reconcile.Result, error) {
	configSecret, pkiSecret, _, err := r.getSecrets(ctx, operation)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get secrets: %w", err)
	}

	if obsolete, reason := r.isOperationObsolete(operation, configSecret, pkiSecret); obsolete {
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

func (r *reconciler) isOperationObsolete(
	operation *controlplanev1alpha1.ControlPlaneOperation,
	configSecret *corev1.Secret,
	pkiSecret *corev1.Secret,
) (bool, string) {
	freshConfig, err := checksum.ComponentChecksum(configSecret.Data, operation.Spec.Component.PodComponentName())
	if err != nil {
		return true, fmt.Sprintf("failed to calculate config checksum: %v", err)
	}

	if operation.Spec.DesiredConfigChecksum != "" && operation.Spec.DesiredConfigChecksum != freshConfig {
		return true, fmt.Sprintf("config checksum changed: desired %s, current %s",
			operation.Spec.DesiredConfigChecksum, freshConfig)
	}

	freshPKI, err := checksum.ComponentPKIChecksum(pkiSecret.Data, operation.Spec.Component.PodComponentName())
	if err != nil {
		return true, fmt.Sprintf("failed to calculate pki checksum: %v", err)
	}

	if operation.Spec.DesiredPKIChecksum != "" && operation.Spec.DesiredPKIChecksum != freshPKI {
		return true, fmt.Sprintf("pki checksum changed: desired %s, current %s",
			operation.Spec.DesiredPKIChecksum, freshPKI)
	}

	freshCA, err := checksum.PKIChecksum(pkiSecret.Data)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate ca checksum: %v", err)
	}

	if operation.Spec.DesiredCAChecksum != "" && operation.Spec.DesiredCAChecksum != freshCA {
		return true, fmt.Sprintf(
			"ca checksum changed: desired %s, current %s",
			operation.Spec.DesiredCAChecksum,
			freshCA)
	}

	return false, ""
}

func (r *reconciler) reconcileAbandonedOperation(
	ctx context.Context,
	operation *controlplanev1alpha1.ControlPlaneOperation,
	reason string,
) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyAbandonedOperation(operation, reason)
	return reconcile.Result{}, r.patchOperationStatus(ctx, operation, base)
}

func applyAbandonedOperation(operation *controlplanev1alpha1.ControlPlaneOperation, message string) {
	setCondition(
		operation,
		controlplanev1alpha1.CPOConditionCompleted,
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonOperationAbandoned,
		message,
	)
}

func (r *reconciler) reconcileFailedOperation(
	ctx context.Context,
	operation *controlplanev1alpha1.ControlPlaneOperation,
	result operations.OperationResult,
) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyStepResults(operation, result.StepResults)
	applyOperationFailed(operation, result.Message)

	if err := r.patchOperationStatus(ctx, operation, base); err != nil {
		log.FromContext(ctx).Error(err, "failed to patch operation status")
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

func (r *reconciler) reconcileInProgressOperation(
	ctx context.Context,
	operation *controlplanev1alpha1.ControlPlaneOperation,
	result operations.OperationResult,
) (reconcile.Result, error) {
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

func (r *reconciler) reconcileCompletedOperation(
	ctx context.Context,
	operation *controlplanev1alpha1.ControlPlaneOperation,
	result operations.OperationResult,
) (reconcile.Result, error) {
	base := operation.DeepCopy()
	applyStepResults(operation, result.StepResults)
	applyOperationFuncs(operation, result.OperationFuncs)
	applyOperationCompleted(operation)

	return reconcile.Result{}, r.patchOperationStatus(ctx, operation, base)
}

func applyOperationFuncs(operation *controlplanev1alpha1.ControlPlaneOperation, funcs []func(operation *controlplanev1alpha1.ControlPlaneOperation)) {
	for _, fn := range funcs {
		fn(operation)
	}
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

func applyStepFailed(
	operation *controlplanev1alpha1.ControlPlaneOperation,
	name controlplanev1alpha1.StepName,
	errorMessage string,
) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonStepFailed,
		errorMessage,
	)
}

func applyStepInProgress(
	operation *controlplanev1alpha1.ControlPlaneOperation,
	name controlplanev1alpha1.StepName,
	message string,
) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionFalse,
		controlplanev1alpha1.CPOReasonStepInProgress,
		message,
	)
}

func applyStepCompleted(
	operation *controlplanev1alpha1.ControlPlaneOperation,
	name controlplanev1alpha1.StepName,
	message string,
) {
	setCondition(
		operation,
		controlplanev1alpha1.StepConditionType(name),
		metav1.ConditionTrue,
		controlplanev1alpha1.CPOReasonStepCompleted,
		message,
	)
}

func (r *reconciler) getOperation(
	ctx context.Context, namespacedName types.NamespacedName,
) (*controlplanev1alpha1.ControlPlaneOperation, error) {
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

func (r *reconciler) getSecrets(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (*corev1.Secret, *corev1.Secret, *corev1.Secret, error) {
	configSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: operation.Namespace,
		Name:      operation.Namespace + constants.VirtualControlPlaneConfigSecretSuffix,
	}, configSecret); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get config secret: %v", err)
	}

	pkiSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: operation.Namespace,
		Name:      operation.Namespace + "-pki",
	}, pkiSecret); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get pki secret: %v", err)
	}

	kubeconfigSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: operation.Namespace,
		Name:      operation.Namespace + "-kubeconfig",
	}, kubeconfigSecret); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get kubeconfig secret: %v", err)
	}

	return configSecret, pkiSecret, kubeconfigSecret, nil
}
