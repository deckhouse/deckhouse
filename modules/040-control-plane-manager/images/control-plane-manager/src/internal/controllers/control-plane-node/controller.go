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
	requeueInterval         = 5 * time.Minute
)

type Reconciler struct {
	client client.Client
	log    *log.Logger
}

func Register(mgr manager.Manager) error {
	nodeName := os.Getenv(constants.NodeNameEnvVar)
	if nodeName == "" {
		return fmt.Errorf("environment variable %s is not set", constants.NodeNameEnvVar)
	}

	r := &Reconciler{
		client: mgr.GetClient(),
		log:    log.Default(),
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
	logger := r.log.With(slog.String("node", nodeName))
	logger.Info("Reconcile started for ControlPlaneNode")

	controlPlaneNode := &controlplanev1alpha1.ControlPlaneNode{}
	err := r.client.Get(ctx, client.ObjectKey{Name: nodeName}, controlPlaneNode)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("ControlPlaneNode not found, skipping")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	logger.Info("ControlPlaneNode found")

	states := buildComponentStates(controlPlaneNode)

	if err := r.ensureOperationsExist(ctx, controlPlaneNode, states, logger); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.updateStatusConditions(ctx, controlPlaneNode, states); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// componentState holds state (spec and status checksums) for a single component.
type componentState struct {
	component      controlplanev1alpha1.OperationComponent
	specChecksum   string
	statusChecksum string
	conditionType  string
}

// ensureOperationsExist compares spec vs status checksums and creates ControlPlaneOperation for each component where they differ.
func (r *Reconciler) ensureOperationsExist(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState, logger *log.Logger) error {
	nodeName := cpn.Name

	operations := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, operations, client.MatchingLabels{
		constants.ControlPlaneNodeNameLabelKey: nodeName,
	}); err != nil {
		return fmt.Errorf("list ControlPlaneOperations for node %s: %w", nodeName, err)
	}

	// Ensures we only skip creation when the operation is already owned by the current CPN instance.
	// If cpn was deleted and recreated, it will have a different UID
	existingOwners := make(map[string]types.UID, len(operations.Items))
	for i := range operations.Items {
		for _, ref := range operations.Items[i].OwnerReferences {
			if ref.Controller != nil && *ref.Controller {
				existingOwners[operations.Items[i].Name] = ref.UID
				break
			}
		}
	}

	for _, state := range states {
		if state.specChecksum == state.statusChecksum {
			continue
		}

		operationName := operationNameForNode(nodeName, state.component, state.specChecksum)
		if ownerUID, exists := existingOwners[operationName]; exists && ownerUID == cpn.UID {
			logger.Debug("ControlPlaneOperation already exists, skipping",
				slog.String("operation", operationName),
				slog.String("component", string(state.component)))
			continue
		}

		operation := newControlPlaneOperation(cpn, operationName, nodeName, state)

		if err := r.client.Create(ctx, operation); err != nil {
			return fmt.Errorf("create ControlPlaneOperation %s: %w", operationName, err)
		}
		logger.Info("ControlPlaneOperation created",
			slog.String("operation", operationName),
			slog.String("component", string(state.component)))
	}

	return nil
}

func buildComponentStates(cpn *controlplanev1alpha1.ControlPlaneNode) []componentState {
	return []componentState{
		{
			component:      controlplanev1alpha1.OperationComponentEtcd,
			conditionType:  constants.ConditionEtcdReady,
			specChecksum:   cpn.Spec.Components.Etcd.Checksum,
			statusChecksum: cpn.Status.Components.Etcd.Checksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeAPIServer,
			conditionType:  constants.ConditionAPIServerReady,
			specChecksum:   cpn.Spec.Components.KubeAPIServer.Checksum,
			statusChecksum: cpn.Status.Components.KubeAPIServer.Checksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeControllerManager,
			conditionType:  constants.ConditionControllerManagerReady,
			specChecksum:   cpn.Spec.Components.KubeControllerManager.Checksum,
			statusChecksum: cpn.Status.Components.KubeControllerManager.Checksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentKubeScheduler,
			conditionType:  constants.ConditionSchedulerReady,
			specChecksum:   cpn.Spec.Components.KubeScheduler.Checksum,
			statusChecksum: cpn.Status.Components.KubeScheduler.Checksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentHotReload,
			conditionType:  constants.ConditionHotReloadSynced,
			specChecksum:   cpn.Spec.HotReloadChecksum,
			statusChecksum: cpn.Status.HotReloadChecksum,
		},
		{
			component:      controlplanev1alpha1.OperationComponentPKI,
			conditionType:  constants.ConditionPKISynced,
			specChecksum:   cpn.Spec.PKIChecksum,
			statusChecksum: cpn.Status.PKIChecksum,
		},
	}
}

func newControlPlaneOperation(cpn *controlplanev1alpha1.ControlPlaneNode, operationName, nodeName string, state componentState) *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name: operationName,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  nodeName,
				constants.ControlPlaneComponentLabelKey: string(state.component),
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
			Component:       state.component,
			Command:         controlplanev1alpha1.OperationCommandUpdate,
			DesiredChecksum: state.specChecksum,
			Approved:        false,
		},
	}
}

// operationNameForNode returns a deterministic k8s like resource name for ControlPlaneOperation <node-name>-<component>-<checksum>.
func operationNameForNode(nodeName string, component controlplanev1alpha1.OperationComponent, specChecksum string) string {
	sanitized := strings.ReplaceAll(nodeName, ".", "-")
	if len(specChecksum) > 6 {
		specChecksum = specChecksum[:6]
	}
	return fmt.Sprintf("%s-%s-%s", sanitized, strings.ToLower(string(component)), specChecksum)
}

// TODO: Add controlPlaneOperation based conditions logic.
func newCondition(condType string, specChecksum, statusChecksum string, generation int64) metav1.Condition {
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

func (r *Reconciler) updateStatusConditions(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState) error {
	original := cpn.DeepCopy()

	for _, state := range states {
		cond := newCondition(state.conditionType, state.specChecksum, state.statusChecksum, cpn.Generation)
		meta.SetStatusCondition(&cpn.Status.Conditions, cond)
	}

	if reflect.DeepEqual(original.Status.Conditions, cpn.Status.Conditions) {
		return nil
	}

	return r.client.Status().Patch(ctx, cpn, client.MergeFrom(original))
}
