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

package virtualcontrolplanenode

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	cpnplanner "control-plane-manager/internal/cpn"
)

const (
	requeueInterval                   = 5 * time.Minute
	maxTerminalOperationsPerComponent = 5
)

type reconciler struct {
	client client.Client
	// apiReader is an uncached reader used to confirm, right before creating an operation, that the previous reconcile of the same node did not already create it.
	apiReader client.Reader
	scheme    *runtime.Scheme
}

var _ reconcile.Reconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	if err := r.client.Get(ctx, req.NamespacedName, cpn); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	current, err := r.listOperationsForNode(ctx, cpn)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileStatus(ctx, cpn, current); err != nil {
		return reconcile.Result{}, err
	}

	if cpnplanner.IsMaintenanceMode(cpn) {
		// using requeueInterval for observation removing maintenance mode label.
		return reconcile.Result{RequeueAfter: requeueInterval}, nil
	}

	if err := r.reconcileOperations(ctx, cpn, current); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileRotation(ctx, cpn, current); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// reconcileStatus folds completed operations into the node status (conditions, applied checksums, cert dates).
func (r *reconciler) reconcileStatus(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) error {
	target := cpnplanner.ComputeStatusReport(cpn, ops)
	if equality.Semantic.DeepEqual(cpn.Status, target) {
		return nil
	}
	base := cpn.DeepCopy()
	cpn.Status = target
	return r.client.Status().Patch(ctx, cpn, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

// reconcileOperations creates CPOs for components that drifted from the desired state.
//
// Deduplication is done first against the informer cache. Only when that decides something must be created do we re-check against a strongly-consistent uncached read.
// This prevents duplicates without paying the uncached read on steady-state reconciles.
func (r *reconciler) reconcileOperations(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation) error {
	if len(cpnplanner.BuildOperations(cpn, current)) == 0 {
		return nil // nothing to create: no uncached read on steady-state reconciles
	}

	fresh, err := r.listOperationsForNodeUncached(ctx, cpn)
	if err != nil {
		return err
	}
	for _, op := range cpnplanner.BuildOperations(cpn, fresh) {
		if err := r.createOperation(ctx, cpn, op); err != nil {
			return err
		}
	}
	return nil
}

// reconcileRotation deletes terminal operations beyond the per-component retention limit.
func (r *reconciler) reconcileRotation(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation) error {
	for _, name := range cpnplanner.ComputeOperationsToRotate(current, maxTerminalOperationsPerComponent) {
		if err := r.deleteOperation(ctx, cpn.Namespace, name); err != nil {
			return err
		}
	}
	return nil
}

// listOperationsForNode lists the node's operations from the informer cache (used for status and rotation).
func (r *reconciler) listOperationsForNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	return r.listOperations(ctx, r.client, cpn)
}

// listOperationsForNodeUncached lists the node's operations directly from the API server (strongly consistent).
func (r *reconciler) listOperationsForNodeUncached(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	return r.listOperations(ctx, r.apiReader, cpn)
}

func (r *reconciler) listOperations(ctx context.Context, reader client.Reader, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	list := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := reader.List(ctx, list,
		client.InNamespace(cpn.Namespace),
		client.MatchingLabels{constants.ControlPlaneNodeNameLabelKey: cpn.Name},
	); err != nil {
		return nil, err
	}
	// Keep only operations owned by this exact CPN (name + UID): prevents reconstructing state from a previous same-name instance's not yet garbage collected operations after CPN recreation.
	return cpnplanner.FilterOperationsOwnedByCPN(list.Items, cpn), nil
}

func (r *reconciler) createOperation(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) error {
	if err := controllerutil.SetControllerReference(cpn, op, r.scheme); err != nil {
		return err
	}
	return r.client.Create(ctx, op)
}

func (r *reconciler) deleteOperation(ctx context.Context, namespace, name string) error {
	op := &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
	}
	return client.IgnoreNotFound(r.client.Delete(ctx, op))
}
