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
	"time"

	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
)

const (
	maxConcurrentReconciles = 1
	cacheSyncTimeout        = 3 * time.Minute
	requeueWaitPod          = 5 * time.Second
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client  client.Client
	log     *log.Logger
	node    NodeIdentity
	steps   map[controlplanev1alpha1.StepName]Step
	metrics *metrics
}

func Register(mgr manager.Manager, metricsStorage metricsstorage.Storage) error {
	node, err := nodeIdentityFromEnv()
	if err != nil {
		return fmt.Errorf("read node identity: %w", err)
	}

	metricHandlers, err := newMetrics(metricsStorage)
	if err != nil {
		return fmt.Errorf("init metrics: %w", err)
	}

	r := &Reconciler{
		client:  mgr.GetClient(),
		log:     log.Default().With(slog.String("controller", constants.CpoControllerName)),
		node:    node,
		steps:   defaultSteps(),
		metrics: metricHandlers,
	}
	// Inject Reconciler-level deps into steps that need them.
	r.steps[controlplanev1alpha1.StepWaitPodReady].(*waitPodReadyStep).waitForPod = r.waitForPod

	// harden admin kubeconfig perms and align root kubeconfig symlink during controller startup.
	r.enforceNodePolicy(r.log)

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
				workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](5*time.Second, 30*time.Minute),
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

// keep named return to log deferred reconcile result
//
//nolint:nonamedreturns
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	logger := r.log.With(slog.String("operation", req.Name))

	op := &controlplanev1alpha1.ControlPlaneOperation{}
	if err := r.client.Get(ctx, req.NamespacedName, op); err != nil {
		if apierrors.IsNotFound(err) {
			r.metrics.deleteOperationExecutionMetrics(req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	defer func() {
		r.metrics.syncOperationExecutionMetrics(op)
	}()

	state := controlplanev1alpha1.NewOperationState(op)

	// Initialize default unknown conditions
	if !op.IsTerminal() {
		state.EnsureInitialConditions()
		if err := r.patchStatus(ctx, state); err != nil {
			return reconcile.Result{}, fmt.Errorf("initialize conditions: %w", err)
		}
	}

	if !op.Spec.Approved || op.IsTerminal() {
		return reconcile.Result{}, nil
	}

	if err := r.ensureOperationStartedAt(ctx, op, time.Now()); err != nil {
		return reconcile.Result{}, fmt.Errorf("set operation started timestamp: %w", err)
	}

	logger.Info("reconciling operation",
		slog.String("component", string(op.Spec.Component)),
		slog.Any("steps", op.Spec.Steps))
	defer func() {
		if err != nil {
			logger.Error("reconcile failed", log.Err(err))
		} else {
			logger.Info("reconcile finished")
		}
	}()

	// Observe-only operations are read-only, no secrets needed.
	if op.IsObserveOnlyOperation() {
		return r.reconcilePipeline(ctx, state, ClusterSecrets{}, logger)
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
	secrets := ClusterSecrets{CPMData: cpmSecret.Data, PKIData: pkiSecret.Data}

	// Verify that the secret content matches what this operation was created for.
	if stale, reason := isDesiredStale(op, secrets); stale {
		completion, completionErr := r.markInProgressCommitPointCompletedIfApplied(ctx, state)
		if completionErr != nil {
			return reconcile.Result{}, completionErr
		} else if completion.Applied {
			logger.Info("recovered in-progress commit-point step from disk state", slog.String("step", string(completion.Step)))
		}

		logger.Info("desired checksums stale, operation abandoned", slog.String("reason", reason))
		state.MarkOperationAbandoned(reason)
		return reconcile.Result{}, r.patchStatus(ctx, state)
	}

	return r.reconcilePipeline(ctx, state, secrets, logger)
}

// enforceNodePolicy applies node security policy during controller startup:
// keep admin kubeconfig perms at 0600 and align root kubeconfig symlink.
func (r *Reconciler) enforceNodePolicy(logger *log.Logger) {
	if err := hardenAdminKubeconfigs(r.node.KubeconfigDir); err != nil {
		logger.Warn("failed to harden admin kubeconfigs", log.Err(err))
	}
	if err := updateRootKubeconfig(r.node.KubeconfigDir, r.node.HomeDir, r.node.NodeAdminKubeconfig); err != nil {
		logger.Warn("failed to enforce root kubeconfig symlink", log.Err(err))
	}
}

func (r *Reconciler) ensureOperationStartedAt(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation, now time.Time) error {
	if op.Annotations != nil && op.Annotations[constants.OperationStartedAtAnnotationKey] != "" {
		return nil
	}

	original := op.DeepCopy()
	if op.Annotations == nil {
		op.Annotations = make(map[string]string, 1)
	}
	op.Annotations[constants.OperationStartedAtAnnotationKey] = now.UTC().Format(time.RFC3339Nano)

	return r.client.Patch(ctx, op, client.MergeFrom(original))
}

// isDesiredStale checks that secret content still matches with desired checksums in the operation spec.
// Returns true with reason string if stale.
func isDesiredStale(op *controlplanev1alpha1.ControlPlaneOperation, secrets ClusterSecrets) (bool, string) {
	component := op.Spec.Component

	podName := component.PodComponentName()

	freshConfig, err := checksum.ComponentChecksum(secrets.CPMData, podName)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate config checksum: %v", err)
	}
	if op.Spec.DesiredConfigChecksum != "" && op.Spec.DesiredConfigChecksum != freshConfig {
		return true, fmt.Sprintf("config checksum changed: desired %s, current %s",
			op.Spec.DesiredConfigChecksum, freshConfig)
	}

	freshPKI, err := checksum.ComponentPKIChecksum(secrets.CPMData, podName)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate pki checksum: %v", err)
	}
	if op.Spec.DesiredPKIChecksum != "" && op.Spec.DesiredPKIChecksum != freshPKI {
		return true, fmt.Sprintf("pki checksum changed: desired %s, current %s",
			op.Spec.DesiredPKIChecksum, freshPKI)
	}

	freshCA, err := checksum.PKIChecksum(secrets.PKIData)
	if err != nil {
		return true, fmt.Sprintf("failed to calculate ca checksum: %v", err)
	}
	if op.Spec.DesiredCAChecksum != "" && op.Spec.DesiredCAChecksum != freshCA {
		return true, fmt.Sprintf("ca checksum changed: desired %s, current %s",
			op.Spec.DesiredCAChecksum, freshCA)
	}

	return false, ""
}

type CommitPointCompletionResult struct {
	Step    controlplanev1alpha1.StepName
	Applied bool
}

func (r *Reconciler) markInProgressCommitPointCompletedIfApplied(ctx context.Context, state *controlplanev1alpha1.OperationState) (CommitPointCompletionResult, error) {
	op := state.Raw()
	step, ok := inProgressCommitPoint(op)
	if !ok {
		return CommitPointCompletionResult{}, nil
	}

	matches, err := r.diskMatchesDesired(op, step)
	if err != nil {
		return CommitPointCompletionResult{}, fmt.Errorf("check disk state for %s: %w", step, err)
	}
	if !matches {
		return CommitPointCompletionResult{Step: step, Applied: false}, nil
	}

	state.MarkStepCompleted(step)
	if err := r.patchStatus(ctx, state); err != nil {
		return CommitPointCompletionResult{}, fmt.Errorf("persist recovered step %s: %w", step, err)
	}
	return CommitPointCompletionResult{Step: step, Applied: true}, nil
}

func inProgressCommitPoint(op *controlplanev1alpha1.ControlPlaneOperation) (controlplanev1alpha1.StepName, bool) {
	switch {
	case op.IsStepInProgress(controlplanev1alpha1.StepSyncManifests):
		return controlplanev1alpha1.StepSyncManifests, true
	case op.IsStepInProgress(controlplanev1alpha1.StepJoinEtcdCluster):
		return controlplanev1alpha1.StepJoinEtcdCluster, true
	default:
		return "", false
	}
}

func (r *Reconciler) diskMatchesDesired(op *controlplanev1alpha1.ControlPlaneOperation, step controlplanev1alpha1.StepName) (bool, error) {
	switch step {
	case controlplanev1alpha1.StepSyncManifests:
		return manifestMatchesDesired(op)
	case controlplanev1alpha1.StepJoinEtcdCluster:
		manifestMatches, err := manifestMatchesDesired(op)
		if err != nil || !manifestMatches {
			return manifestMatches, err
		}
		peerURL := etcd.GetPeerURL(r.node.AdvertiseIP)
		memberExists, err := checkEtcdMemberExists(r.node.Name, peerURL, constants.KubernetesPkiPath, r.node.KubeconfigDir)
		if err != nil {
			return false, err
		}
		return memberExists, nil
	default:
		return false, nil
	}
}
