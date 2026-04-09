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
	"reflect"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
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
	node     NodeIdentity
	commands map[controlplanev1alpha1.CommandName]Command
}

func Register(mgr manager.Manager) error {
	node, err := nodeIdentityFromEnv()
	if err != nil {
		return fmt.Errorf("read node identity: %w", err)
	}

	r := &Reconciler{
		client:   mgr.GetClient(),
		log:      log.Default(),
		node:     node,
		commands: defaultCommands(),
	}
	// Inject Reconciler-level deps into commands that need them.
	r.commands[controlplanev1alpha1.CommandWaitPodReady].(*waitPodReadyCommand).pods = r

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: node.Name,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	cpoPredicate := predicate.And(nodeLabelPredicate, approvedCPOPredicate())
	podPredicate := controlPlanePodPredicate(node.Name)

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

// approvedCPOPredicate triggers on CPO that become Approved.
func approvedCPOPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			op, ok := e.Object.(*controlplanev1alpha1.ControlPlaneOperation)
			return ok && op.Spec.Approved
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
			newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
			return okOld && okNew && !oldOp.Spec.Approved && newOp.Spec.Approved
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

// controlPlanePodPredicate triggers on pod condition/annotation changes for control-plane pods on given node.
func controlPlanePodPredicate(nodeName string) predicate.Funcs {
	return predicate.Funcs{
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

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	logger := r.log.With(slog.String("operation", req.Name))

	op := &controlplanev1alpha1.ControlPlaneOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	if !op.Spec.Approved || op.IsTerminal() {
		return reconcile.Result{}, nil
	}

	state := controlplanev1alpha1.NewOperationState(op)

	logger.Info("reconciling operation",
		slog.String("component", string(op.Spec.Component)),
		slog.Any("commands", op.Spec.Commands))
	defer func() {
		if err != nil {
			logger.Error("reconcile failed", log.Err(err))
		} else {
			logger.Info("reconcile finished")
		}
	}()

	// CertObserver read only, no secrets or configVersion needed
	if op.Spec.Component == controlplanev1alpha1.OperationComponentCertObserver {
		return r.reconcilePipeline(ctx, state, nil, nil, logger)
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
		state.SetReadyReason(constants.ReasonCancelled,
			fmt.Sprintf("configVersion mismatch: operation has %s, current is %s", op.Spec.ConfigVersion, currentConfigVersion))
		return reconcile.Result{}, r.patchStatus(ctx, state)
	}

	return r.reconcilePipeline(ctx, state, cpmSecret.Data, pkiSecret.Data, logger)
}

// reconcilePipeline executes the command-based pipeline for component operations.
// Completed commands (condition=True) are skipped on requeue.
// On command failure the failed command condition stays False, so it re-executes on next reconcile.
func (r *Reconciler) reconcilePipeline(ctx context.Context, state *controlplanev1alpha1.OperationState, cpmSecretData, pkiSecretData map[string][]byte, logger *log.Logger) (reconcile.Result, error) {
	commands, err := resolveCommands(r.commands, state.Raw().Spec.Commands)
	if err != nil {
		return reconcile.Result{}, err
	}

	env := &CommandEnv{
		State:         state,
		CPMSecretData: cpmSecretData,
		PKISecretData: pkiSecretData,
		Node:          r.node,
		FlushStatus:   func(ctx context.Context) error { return r.patchStatus(ctx, state) },
	}

	for _, cmd := range commands {
		result, err := r.executeCommand(ctx, state, cmd, env, logger)
		if err != nil {
			return result, err
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	// All commands completed successfully — mark operation as ready.
	// For static pod components this is typically done by WaitPodReady,
	// but for CA/HotReload the pipeline may not include WaitPodReady.
	if !state.IsCompleted() {
		state.MarkSucceeded()
		return reconcile.Result{}, r.patchStatus(ctx, state)
	}

	return reconcile.Result{}, nil
}

// executeCommand runs a single pipeline command with status tracking and start/finish logging.
func (r *Reconciler) executeCommand(ctx context.Context, state *controlplanev1alpha1.OperationState, cmd Command, env *CommandEnv, logger *log.Logger) (result reconcile.Result, err error) {
	name := cmd.CommandName()
	cmdLogger := logger.With(slog.String("command", string(name)))

	if state.IsCommandCompleted(name) {
		cmdLogger.Info("command already completed, skipping")
		return reconcile.Result{}, nil
	}

	cmdLogger.Info("executing command")
	defer func() {
		if err != nil {
			cmdLogger.Error("command failed", log.Err(err))
		} else {
			cmdLogger.Info("command finished")
		}
	}()

	state.MarkCommandInProgress(name)
	state.SetReadyReason(cmd.ReadyReason(), fmt.Sprintf("executing command %s", name))
	if patchErr := r.patchStatus(ctx, state); patchErr != nil {
		cmdLogger.Warn("failed to set in-progress condition", log.Err(patchErr))
	}

	result, err = cmd.Execute(ctx, env, cmdLogger)
	if err != nil {
		state.MarkCommandFailed(name, err.Error())
		if setErr := r.patchStatus(ctx, state); setErr != nil {
			cmdLogger.Error("failed to set failed condition", log.Err(setErr))
		}
		return result, err
	}

	if result.RequeueAfter > 0 {
		return result, nil
	}

	state.MarkCommandCompleted(name)
	if err = r.patchStatus(ctx, state); err != nil {
		return result, fmt.Errorf("set completed condition for %s: %w", name, err)
	}
	return result, nil
}

// waitForPod checks if the static pod is ready with the expected checksums annotations.
func (r *Reconciler) waitForPod(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error) {
	op := state.Raw()
	podName := fmt.Sprintf("%s-%s", op.Spec.Component.PodComponentName(), r.node.Name)
	pod := &corev1.Pod{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: podName, Namespace: constants.KubeSystemNamespace}, pod); err != nil {
		logger.Info("pod not found yet, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if isPodCrashLooping(pod) {
		logger.Warn("pod is crash looping, will retry", slog.String("pod", podName))
		state.SetReadyReason(constants.ReasonWaitingForPod,
			fmt.Sprintf("pod %s is in CrashLoopBackOff, will retry", podName))
		_ = r.patchStatus(ctx, state)
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	if !isPodReadyWithChecksums(pod,
		op.Spec.DesiredConfigChecksum,
		op.Spec.DesiredPKIChecksum,
		op.Spec.DesiredCAChecksum,
		op.CertRenewalID(),
	) {
		logger.Info("pod not ready with expected checksums, requeue", slog.String("pod", podName))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}

	logger.Info("pod ready with matching checksums", slog.String("pod", podName))

	state.MarkSucceeded()
	return reconcile.Result{}, r.patchStatus(ctx, state)
}

// patchStatus flushes OperationState status changes to the API server.
func (r *Reconciler) patchStatus(ctx context.Context, state *controlplanev1alpha1.OperationState) error {
	if !state.HasStatusChanges() {
		return nil
	}
	if err := r.client.Status().Patch(ctx, state.Raw(), client.MergeFrom(state.Original())); err != nil {
		return err
	}
	state.ResetBaseline()
	return nil
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
		constants.ControlPlaneNodeNameLabelKey:  r.node.Name,
		constants.ControlPlaneComponentLabelKey: string(opComponent),
	}); err != nil {
		return nil
	}

	var reqs []reconcile.Request
	for i := range ops.Items {
		if ops.Items[i].Spec.Approved && !ops.Items[i].IsCompleted() {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: ops.Items[i].Name},
			})
		}
	}
	return reqs
}

func shortChecksum(checksum string) string {
	if len(checksum) > 8 {
		return checksum[:8]
	}
	return checksum
}
