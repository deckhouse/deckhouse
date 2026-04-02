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
	"context"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueWaitPod          = 5 * time.Second
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client   client.Client
	log      *log.Logger
	nodeName string
}

func Register(mgr manager.Manager) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("env %s is not set", constants.NodeNameEnvVar)
	}

	r := &Reconciler{
		client:   mgr.GetClient(),
		log:      log.Default(),
		nodeName: nodeName,
	}

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: nodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	// React to CPO when Approved set to true
	cpoPredicate := predicate.And(
		nodeLabelPredicate,
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				op, ok := e.Object.(*controlplanev1alpha1.ControlPlaneOperation)
				return ok && op.Spec.Approved
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
				newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
				if !okOld || !okNew {
					return false
				}
				return !oldOp.Spec.Approved && newOp.Spec.Approved
			},
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		},
	)

	// React to pod status/annotation changes for control-plane pods on this node
	podPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isNodeControlPlanePod(e.Object, nodeName)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if !isNodeControlPlanePod(e.ObjectNew, nodeName) {
				return false
			}
			oldPod, okOld := e.ObjectOld.(*corev1.Pod)
			newPod, okNew := e.ObjectNew.(*corev1.Pod)
			if !okOld || !okNew {
				return false
			}
			return !reflect.DeepEqual(oldPod.Status.Conditions, newPod.Status.Conditions) ||
				oldPod.Annotations[constants.ConfigChecksumAnnotationKey] != newPod.Annotations[constants.ConfigChecksumAnnotationKey] ||
				oldPod.Annotations[constants.PKIChecksumAnnotationKey] != newPod.Annotations[constants.PKIChecksumAnnotationKey] ||
				oldPod.Annotations[constants.CAChecksumAnnotationKey] != newPod.Annotations[constants.CAChecksumAnnotationKey]
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			CacheSyncTimeout:        cacheSyncTimeout,
			NeedLeaderElection:      ptr.To(false),
			RateLimiter: workqueue.NewTypedMaxOfRateLimiter(
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](100*time.Millisecond, 3*time.Second),
				&workqueue.TypedBucketRateLimiter[reconcile.Request]{
					Limiter: rate.NewLimiter(rate.Limit(1), 1),
				},
			),
		}).
		Named(constants.CpoControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(cpoPredicate),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.mapPodToOperations),
			builder.WithPredicates(podPredicate),
		).
		Complete(r)
}

func isNodeControlPlanePod(obj client.Object, nodeName string) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}
	if pod.Namespace != constants.KubeSystemNamespace {
		return false
	}
	if pod.Spec.NodeName != nodeName {
		return false
	}
	component := pod.Labels[constants.StaticPodComponentLabelKey]
	_, known := controlplanev1alpha1.OperationComponentFromPodName(component)
	return known
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.log.With(slog.String("operation", req.Name))

	op := &controlplanev1alpha1.ControlPlaneOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !op.Spec.Approved {
		return reconcile.Result{}, nil
	}

	if isCompleted(op) || isFailed(op) {
		return reconcile.Result{}, nil
	}

	logger.Info("reconciling operation",
		slog.String("component", string(op.Spec.Component)),
		slog.Any("commands", op.Spec.Commands))

	// Observe read only, no secrets or configVersion needed
	if op.Spec.Component == controlplanev1alpha1.OperationComponentObserver {
		return r.reconcilePipeline(ctx, op, nil, nil, logger)
	}

	cpmSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      constants.ControlPlaneManagerConfigSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, cpmSecret); err != nil {
		return reconcile.Result{}, fmt.Errorf("get cpm secret: %w", err)
	}

	pkiSecret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{
		Name:      constants.PkiSecretName,
		Namespace: constants.KubeSystemNamespace,
	}, pkiSecret); err != nil {
		return reconcile.Result{}, fmt.Errorf("get pki secret: %w", err)
	}

	// Validate configVersion - same for all commands(types)
	currentConfigVersion := fmt.Sprintf("%s.%s", cpmSecret.ResourceVersion, pkiSecret.ResourceVersion)
	if op.Spec.ConfigVersion != currentConfigVersion {
		logger.Info("configVersion mismatch, cancelling",
			slog.String("expected", op.Spec.ConfigVersion),
			slog.String("actual", currentConfigVersion))
		return reconcile.Result{}, r.setConditions(ctx, op,
			readyCondition(metav1.ConditionFalse,
				constants.ReasonCancelled,
				fmt.Sprintf("configVersion mismatch: operation has %s, current is %s", op.Spec.ConfigVersion, currentConfigVersion)))
	}

	return r.reconcilePipeline(ctx, op, cpmSecret.Data, pkiSecret.Data, logger)
}

// reconcilePipeline executes the command-based pipeline for component operations.
// Completed commands (condition=True) are skipped on requeue
// On command failure the failed command condition stays False, so it re-executes on next reconcile.
func (r *Reconciler) reconcilePipeline(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, cpmSecretData, pkiSecretData map[string][]byte, logger *log.Logger) (reconcile.Result, error) {
	commands, err := resolveCommands(op.Spec.Commands)
	if err != nil {
		return reconcile.Result{}, err
	}

	cc := &commandContext{
		r:              r,
		op:             op,
		component:      op.Spec.Component,
		cpmSecretData:  cpmSecretData,
		pkiSecretData:  pkiSecretData,
		configChecksum: op.Spec.DesiredConfigChecksum,
		pkiChecksum:    op.Spec.DesiredPKIChecksum,
		caChecksum:     op.Spec.DesiredCAChecksum,
	}

	//Skip already completed commands here (testing purposes)
	for _, cmd := range commands {
		if meta.IsStatusConditionTrue(op.Status.Conditions, string(cmd.Name)) {
			logger.Info("command already completed, skipping", slog.String("command", string(cmd.Name)))
			continue
		}

		cmdLogger := logger.With(slog.String("command", string(cmd.Name)))
		cmdLogger.Info("executing command")

		_ = r.setConditions(ctx, op,
			commandCondition(cmd.Name, metav1.ConditionFalse, constants.ReasonCommandInProgress, ""),
			readyCondition(metav1.ConditionFalse, cmd.ReadyReason,
				fmt.Sprintf("executing command %s", cmd.Name)))

		result, err := cmd.Exec(ctx, cc, cmdLogger)
		if err != nil {
			_ = r.setConditions(ctx, op,
				commandCondition(cmd.Name, metav1.ConditionFalse, constants.ReasonCommandFailed, err.Error()))
			return result, err
		}

		_ = r.setConditions(ctx, op,
			commandCondition(cmd.Name, metav1.ConditionTrue, constants.ReasonCommandCompleted, ""))

		// Command wants requeue (waitForPod, etcdJoin) — stop pipeline, resume on next reconcile.
		if result.Requeue || result.RequeueAfter > 0 {
			return result, nil
		}
	}

	// All commands completed successfully — mark operation as ready.
	// For static pod components this is typically done by WaitPodReady,
	// but for CA/HotReload the pipeline may not include WaitPodReady.
	if !isCompleted(op) {
		return reconcile.Result{}, r.setConditions(ctx, op,
			readyCondition(metav1.ConditionTrue, constants.ReasonOperationSucceeded, "operation completed"),
			failedCondition(metav1.ConditionFalse, constants.ReasonNoFailure, ""),
		)
	}

	return reconcile.Result{}, nil
}

// waitForPod checks if the static pod is ready with the expected checksums annotations.
func (r *Reconciler) waitForPod(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, configChecksum, pkiChecksum, caChecksum string, logger *log.Logger) (reconcile.Result, error) {
	podName := fmt.Sprintf("%s-%s", op.Spec.Component.PodComponentName(), r.nodeName)
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: constants.KubeSystemNamespace}, pod); err != nil {
		logger.Info("pod not found yet, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if isPodCrashLooping(pod) {
		logger.Warn("pod is crash looping, will retry", slog.String("pod", podName))
		_ = r.setConditions(ctx, op,
			readyCondition(metav1.ConditionFalse, constants.ReasonWaitingForPod,
				fmt.Sprintf("pod %s is in CrashLoopBackOff, will retry", podName)))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if !isPodReadyWithChecksums(pod, configChecksum, pkiChecksum, caChecksum) {
		logger.Info("pod not ready with expected checksums, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	logger.Info("pod ready with matching checksums", slog.String("pod", podName))

	return reconcile.Result{}, r.setConditions(ctx, op,
		readyCondition(metav1.ConditionTrue, constants.ReasonOperationSucceeded, "operation completed"),
		failedCondition(metav1.ConditionFalse, constants.ReasonNoFailure, ""),
	)
}

// mapPodToOperations finds in-progress CPOs for the component matching this pod.
func (r *Reconciler) mapPodToOperations(ctx context.Context, obj client.Object) []reconcile.Request {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return nil
	}

	componentName := pod.Labels[constants.StaticPodComponentLabelKey]
	if componentName == "" {
		return nil
	}

	opComponent, ok := controlplanev1alpha1.OperationComponentFromPodName(componentName)
	if !ok {
		return nil
	}

	ops := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, ops, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey:  r.nodeName,
		constants.ControlPlaneComponentLabelKey: string(opComponent),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for i := range ops.Items {
		if ops.Items[i].Spec.Approved && !isCompleted(&ops.Items[i]) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ops.Items[i].Name},
			})
		}
	}
	return reqs
}

// kubeconfigDirPath returns the kubeconfig output directory, allowing override via env var for dev/testing.
func kubeconfigDirPath() string {
	if dir := os.Getenv(constants.KubeconfigDirEnvVar); dir != "" {
		return dir
	}
	return constants.KubernetesConfigPath
}

// condition helpers

func readyCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    constants.ConditionReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func commandCondition(name controlplanev1alpha1.CommandName, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    string(name),
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func failedCondition(status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:    constants.ConditionFailed,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func (r *Reconciler) setConditions(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, conditions ...metav1.Condition) error {
	original := op.DeepCopy()
	for _, c := range conditions {
		meta.SetStatusCondition(&op.Status.Conditions, c)
	}
	if reflect.DeepEqual(original.Status.Conditions, op.Status.Conditions) {
		return nil
	}
	return r.client.Status().Patch(ctx, op, client.MergeFrom(original))
}

func isCompleted(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, constants.ConditionReady)
}

func isFailed(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return meta.IsStatusConditionTrue(op.Status.Conditions, constants.ConditionFailed)
}

func shortChecksum(checksum string) string {
	if len(checksum) > 8 {
		return checksum[:8]
	}
	return checksum
}
