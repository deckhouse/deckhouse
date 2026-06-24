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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpnplanner"
)

const requeueInterval = 5 * time.Minute

type reconciler struct {
	client client.Client
	// apiReader is an uncached reader used to confirm, right before creating an operation, that the previous reconcile of the same node did not already create it.
	apiReader   client.Reader
	scheme      *runtime.Scheme
	stepBuilder cpnplanner.StepBuilder
}

var _ reconcile.Reconciler = (*reconciler)(nil)

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	cpn := &controlplanev1alpha1.ControlPlaneNode{}
	if err := r.client.Get(ctx, req.NamespacedName, cpn); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	current, err := r.listOperationsForNode(ctx, cpn)
	if err != nil {
		return reconcile.Result{}, err
	}

	plan := cpnplanner.ComputePlan(cpn, current, r.stepBuilder)

	if err := r.reconcileStatus(ctx, cpn, plan.Status); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.reconcileOperations(ctx, cpn, plan.Create); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.reconcileRotation(ctx, plan.Delete); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: requeueInterval}, nil
}

func (r *reconciler) reconcileStatus(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, target *controlplanev1alpha1.ControlPlaneNodeStatus) error {
	if target == nil {
		return nil
	}
	base := cpn.DeepCopy()
	cpn.Status = *target
	return r.patchStatus(ctx, cpn, base)
}

func (r *reconciler) reconcileOperations(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, planned []*controlplanev1alpha1.ControlPlaneOperation) error {
	if len(planned) == 0 {
		return nil
	}
	fresh, err := r.listOperationsForNodeUncached(ctx, cpn)
	if err != nil {
		return err
	}
	for _, op := range cpnplanner.ComputePlan(cpn, fresh, r.stepBuilder).Create {
		if err := r.createOperation(ctx, cpn, op); err != nil {
			return err
		}
	}
	return nil
}

func (r *reconciler) reconcileRotation(ctx context.Context, toDelete []*controlplanev1alpha1.ControlPlaneOperation) error {
	for _, op := range toDelete {
		if err := r.deleteOperation(ctx, op); err != nil {
			return err
		}
	}
	return nil
}

func (r *reconciler) listOperationsForNode(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode) ([]controlplanev1alpha1.ControlPlaneOperation, error) {
	return r.listOperations(ctx, r.client, cpn)
}

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
	return cpnplanner.OwnedOperations(cpn, list.Items), nil
}

func (r *reconciler) createOperation(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, op *controlplanev1alpha1.ControlPlaneOperation) error {
	if err := controllerutil.SetControllerReference(cpn, op, r.scheme); err != nil {
		return err
	}
	return r.client.Create(ctx, op)
}

func (r *reconciler) patchStatus(ctx context.Context, cpn *controlplanev1alpha1.ControlPlaneNode, base *controlplanev1alpha1.ControlPlaneNode) error {
	return r.client.Status().Patch(ctx, cpn, client.MergeFromWithOptions(base, client.MergeFromWithOptimisticLock{}))
}

func (r *reconciler) deleteOperation(ctx context.Context, op *controlplanev1alpha1.ControlPlaneOperation) error {
	return client.IgnoreNotFound(r.client.Delete(ctx, op))
}
