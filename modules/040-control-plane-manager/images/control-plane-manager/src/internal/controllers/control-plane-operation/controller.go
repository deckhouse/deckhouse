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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	corev1 "k8s.io/api/core/v1"
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
		log:      log.Default().With(slog.String("controller", constants.CpoControllerName)),
		node:     node,
		commands: defaultCommands(),
	}
	// Inject Reconciler-level deps into commands that need them.
	r.commands[controlplanev1alpha1.CommandWaitPodReady].(*waitPodReadyCommand).waitForPod = r.waitForPod

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
