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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

const requeueInterval = 5 * time.Minute

type reconciler struct {
	client client.Client
	scheme *runtime.Scheme
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

	current, err := r.getOperationsForNode(ctx, cpn)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.reconcileStatus(ctx, cpn, current); err != nil {
		return reconcile.Result{}, err
	}

	if isMaintenanceMode(cpn) {
		return reconcile.Result{}, nil
	}

	if err := r.reconcileOperations(ctx, cpn, current); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

// reconcileStatus folds completed operations into the node status (conditions, applied checksums, cert dates).
func (r *reconciler) reconcileStatus(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) error {
	target := computeStatusReport(cpn, ops)
	if equality.Semantic.DeepEqual(cpn.Status, target) {
		return nil
	}
	base := cpn.DeepCopy()
	cpn.Status = target
	return r.client.Status().Patch(ctx, cpn, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

// reconcileOperations creates CPOs for components that drifted from the desired state.
func (r *reconciler) reconcileOperations(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation) error {
	target := buildTargetOperations(cpn)
	for _, op := range selectOperationsToCreate(current, target) {
		if err := r.createOperation(ctx, cpn, op); err != nil {
			return err
		}
	}
	return nil
}

func (r *reconciler) getOperationsForNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	list := &controlplanev1alpha1.ControlPlaneOperationList{}
	if err := r.client.List(ctx, list,
		client.InNamespace(cpn.Namespace),
		client.MatchingLabels{constants.ControlPlaneNodeNameLabelKey: cpn.Name},
	); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *reconciler) createOperation(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) error {
	if err := controllerutil.SetControllerReference(cpn, op, r.scheme); err != nil {
		return err
	}
	return r.client.Create(ctx, op)
}
