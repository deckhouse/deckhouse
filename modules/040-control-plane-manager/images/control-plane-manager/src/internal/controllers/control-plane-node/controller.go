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
	"context"
	"reflect"
	"strings"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
	"golang.org/x/time/rate"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client client.Client
}

func Register(mgr manager.Manager) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	r := &Reconciler{
		client: mgr.GetClient(),
	}

	nodeLabelPredicate, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{
			constants.ControlPlaneNodeNameLabelKey: nodeName,
		},
	})
	if err != nil {
		return fmt.Errorf("create node label predicate: %w", err)
	}

	// Ignore Create, Delete.
	// React only to Update when Status changed (ignore spec-only updates).
	operationPredicate := predicate.And(
		nodeLabelPredicate,
		predicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return false },
			UpdateFunc: func(e event.UpdateEvent) bool {
				oldOp, okOld := e.ObjectOld.(*controlplanev1alpha1.ControlPlaneOperation)
				newOp, okNew := e.ObjectNew.(*controlplanev1alpha1.ControlPlaneOperation)
				if !okOld || !okNew {
					return false
				}
				return !reflect.DeepEqual(oldOp.Status, newOp.Status)
			},
			DeleteFunc:  func(event.DeleteEvent) bool { return false },
			GenericFunc: func(event.GenericEvent) bool { return false },
		},
	)

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
		Named(constants.CpnControllerName).
		Watches(
			&controlplanev1alpha1.ControlPlaneOperation{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &controlplanev1alpha1.ControlPlaneNode{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(operationPredicate),
		).
		Watches(
			&controlplanev1alpha1.ControlPlaneNode{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(nodeLabelPredicate),
		).
		Complete(r)
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	nodeName := req.Name
	log.Info("Reconcile started for ControlPlaneNode", slog.String("node", nodeName))

	controlPlaneNode := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, controlPlaneNode)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("ControlPlaneNode not found, skipping", slog.String("node", nodeName))
			return reconcile.Result{}, nil
		}
		return reconcile.Result{RequeueAfter: requeueInterval}, err
	}

	log.Info("ControlPlaneNode found", slog.String("node", nodeName))

	if err := r.reconcileComponents(ctx, controlPlaneNode); err != nil {
		return reconcile.Result{RequeueAfter: requeueInterval}, err
	}

	if err := r.reconcileConditions(ctx, controlPlaneNode); err != nil {
		return reconcile.Result{RequeueAfter: requeueInterval}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// componentCheck holds spec and status checksums for a single component.
type componentCheck struct {
	component      controlplanev1alpha1.OperationComponent
	specChecksum   string
	statusChecksum string
}

// reconcileComponents compares spec vs status checksums and creates ControlPlaneOperation
// for each component where they differ.
func (r *Reconciler) reconcileComponents(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	nodeName := cpn.Name
	checks := r.buildComponentChecks(cpn)

	for _, check := range checks {
		if check.specChecksum == check.statusChecksum {
			continue
		}

		operationName := operationNameForNode(nodeName, check.component, check.specChecksum)
		existing := &controlplanev1alpha1.ControlPlaneOperation{}
		err := r.client.Get(ctx, client.ObjectKey{Name: operationName}, existing)
		if err == nil {
			log.Debug("ControlPlaneOperation already exists, skipping",
				slog.String("operation", operationName),
				slog.String("component", string(check.component)))
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("get ControlPlaneOperation %s: %w", operationName, err)
		}

		operation := &controlplanev1alpha1.ControlPlaneOperation{
			ObjectMeta: metav1.ObjectMeta{
				Name: operationName,
				Labels: map[string]string{
					constants.ControlPlaneNodeNameLabelKey:  nodeName,
					constants.ControlPlaneComponentLabelKey: string(check.component),
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion:         controlplanev1alpha1.GroupVersion.String(),
						Kind:               "ControlPlaneNode",
						Name:               cpn.Name,
						UID:                cpn.UID,
						Controller:         ptr.To(true),
						BlockOwnerDeletion: ptr.To(false),
					},
				},
			},
			Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
				ConfigVersion:   cpn.Spec.ConfigVersion,
				NodeName:        nodeName,
				Component:       check.component,
				Command:         controlplanev1alpha1.OperationCommandUpdate,
				DesiredChecksum: check.specChecksum,
				Approved:        false,
			},
		}

		if err := r.client.Create(ctx, operation); err != nil {
			return fmt.Errorf("create ControlPlaneOperation %s: %w", operationName, err)
		}
		log.Info("ControlPlaneOperation created",
			slog.String("operation", operationName),
			slog.String("component", string(check.component)))
	}

	return nil
}

func (r *Reconciler) buildComponentChecks(cpn *controlplanev1alpha1.ControlPlaneNode) []componentCheck {
	spec := &cpn.Spec.Components
	status := &cpn.Status.Components

	return []componentCheck{
		{
			component:      controlplanev1alpha1.OperationComponentEtcd,
			specChecksum:   getChecksum(spec.Etcd),
			statusChecksum: getChecksum(status.Etcd),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeAPIServer,
			specChecksum:   getChecksum(spec.KubeAPIServer),
			statusChecksum: getChecksum(status.KubeAPIServer),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeControllerManager,
			specChecksum:   getChecksum(spec.KubeControllerManager),
			statusChecksum: getChecksum(status.KubeControllerManager),
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeScheduler,
			specChecksum:   getChecksum(spec.KubeScheduler),
			statusChecksum: getChecksum(status.KubeScheduler),
		},
		{
			component:      controlplanev1alpha1.OperationComponentHotReload,
			specChecksum:   cpn.Spec.HotReloadChecksum,
			statusChecksum: cpn.Status.HotReloadChecksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentPKI,
			specChecksum:   cpn.Spec.PKIChecksum,
			statusChecksum: cpn.Status.PKIChecksum,
		},
	}
}

func getChecksum(c *controlplanev1alpha1.ComponentChecksum) string {
	if c == nil {
		return ""
	}
	return c.Checksum
}

// operationNameForNode returns a deterministic k8s like resource name for ControlPlaneOperation <node-name>-<component>-<checksum>.
func operationNameForNode(nodeName string, component controlplanev1alpha1.OperationComponent, specChecksum string) string {
	sanitized := strings.ReplaceAll(nodeName, ".", "-")
	if len(specChecksum) > 6 {
		specChecksum = specChecksum[:6]
	}
	return fmt.Sprintf("%s-%s-%s", sanitized, strings.ToLower(string(component)), specChecksum)
}

func buildCondition(condType string, specChecksum, statusChecksum string, generation int64) metav1.Condition {
	if specChecksum == statusChecksum {
		return metav1.Condition{
			Type:               condType,
			Status:             metav1.ConditionTrue,
			Reason:             constants.ReasonSynced,
			ObservedGeneration: generation,
		}
	}
	return metav1.Condition{
		Type:               condType,
		Status:             metav1.ConditionFalse,
		Reason:             constants.ReasonPendingUpdate,
		ObservedGeneration: generation,
	}
}

func (r *Reconciler) reconcileConditions(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) error {
	patch := client.MergeFrom(cpn.DeepCopy())

	checks := []struct {
		condType string
		spec     string
		status   string
	}{
		{constants.ConditionEtcdReady, getChecksum(cpn.Spec.Components.Etcd), getChecksum(cpn.Status.Components.Etcd)},
		{constants.ConditionAPIServerReady, getChecksum(cpn.Spec.Components.KubeAPIServer), getChecksum(cpn.Status.Components.KubeAPIServer)},
		{constants.ConditionControllerManagerReady, getChecksum(cpn.Spec.Components.KubeControllerManager), getChecksum(cpn.Status.Components.KubeControllerManager)},
		{constants.ConditionSchedulerReady, getChecksum(cpn.Spec.Components.KubeScheduler), getChecksum(cpn.Status.Components.KubeScheduler)},
		{constants.ConditionPKISynced, cpn.Spec.PKIChecksum, cpn.Status.PKIChecksum},
		{constants.ConditionsHotReloadSynced, cpn.Spec.HotReloadChecksum, cpn.Status.HotReloadChecksum},
	}

	for _, check := range checks {
		cond := buildCondition(check.condType, check.spec, check.status, cpn.Generation)
		meta.SetStatusCondition(&cpn.Status.Conditions, cond)
	}
	return r.client.Status().Patch(ctx, cpn, patch)
}
