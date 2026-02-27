/*
Copyright 2025 Flant JSC

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

package controller

import (
	"context"
	"fmt"
	"time"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// InstanceReconciler owns only Instance reconciliation flow.
// Machine and Node status flows are handled by dedicated controllers.
type InstanceReconciler struct {
	client.Client
	machineFactory        MachineFactory
	bashibleStatusFactory BashibleStatusFactory
}

const instanceControllerFinalizer = "node-manager.hooks.deckhouse.io/instance-controller"

const instanceRequeueInterval = time.Minute

func SetupInstanceController(mgr ctrl.Manager) error {
	if err := (&InstanceReconciler{
		Client:                mgr.GetClient(),
		machineFactory:        NewMachineFactory(),
		bashibleStatusFactory: NewBashibleStatusFactory(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup instance reconciler: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.machineFactory == nil {
		return fmt.Errorf("machineFactory is required")
	}
	if r.bashibleStatusFactory == nil {
		return fmt.Errorf("bashibleStatusFactory is required")
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("instance").
		For(&deckhousev1alpha2.Instance{}).
		Complete(r)
}

func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("instance", req.Name)

	instance := &deckhousev1alpha2.Instance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	requeue, err := r.reconcileInstance(ctx, instance)
	if err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}
	if requeue {
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	log.V(1).Info("instance reconciled")
	return ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

func (r *InstanceReconciler) reconcileInstance(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	isDeleting := instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero()
	if isDeleting {
		return r.reconcileInstanceDeletion(ctx, instance)
	}

	if err := r.ensureInstanceFinalizer(ctx, instance); err != nil {
		return false, err
	}

	deleted, err := r.reconcileLinkedSourceExistence(ctx, instance)
	if err != nil || deleted {
		return false, err
	}

	if err := r.reconcileBashibleStatus(ctx, instance); err != nil {
		return false, err
	}

	return false, nil
}

func (r *InstanceReconciler) reconcileInstanceDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	machineGone, err := r.reconcileLinkedMachineDeletion(ctx, instance)
	if err != nil {
		return false, err
	}

	if !controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return !machineGone, nil
	}
	if !machineGone {
		return true, nil
	}

	return false, r.removeInstanceFinalizer(ctx, instance)
}

func (r *InstanceReconciler) ensureInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return nil
	}

	updated := instance.DeepCopy()
	controllerutil.AddFinalizer(updated, instanceControllerFinalizer)
	if err := r.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("ensure finalizer on instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (r *InstanceReconciler) removeInstanceFinalizer(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if !controllerutil.ContainsFinalizer(instance, instanceControllerFinalizer) {
		return nil
	}

	updated := instance.DeepCopy()
	controllerutil.RemoveFinalizer(updated, instanceControllerFinalizer)
	if err := r.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("remove finalizer from instance %q: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (r *InstanceReconciler) reconcileLinkedMachineDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return true, nil
	}

	machine, err := r.machineFactory.NewMachineFromRef(ctx, r.Client, ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	return machine.EnsureDeleted(ctx, r.Client)
}

func (r *InstanceReconciler) reconcileLinkedSourceExistence(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	var err error
	var exists bool
	source := getInstanceSource(instance)

	switch source.Type {
	case instanceSourceMachine:
		machine, machineErr := r.machineFactory.NewMachineFromRef(ctx, r.Client, source.MachineRef)
		if machineErr != nil {
			if apierrors.IsNotFound(machineErr) {
				exists = false
				break
			}
			return false, machineErr
		}
		exists, err = machine.Exists(ctx, r.Client)
	case instanceSourceNode:
		exists, err = r.linkedNodeExists(ctx, source.NodeName)
	default:
		return false, nil
	}

	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := r.Delete(ctx, instance); err != nil && !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("delete instance %q with missing source: %w", instance.Name, err)
	}

	return true, nil
}

func (r *InstanceReconciler) reconcileBashibleStatus(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	desiredStatus := r.bashibleStatusFactory.FromConditions(instance.Status.Conditions)
	if instance.Status.BashibleStatus == desiredStatus {
		return nil
	}

	updated := instance.DeepCopy()
	updated.Status.BashibleStatus = desiredStatus
	if err := r.Status().Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return fmt.Errorf("patch instance %q bashible status: %w", instance.Name, err)
	}

	*instance = *updated
	return nil
}

func (r *InstanceReconciler) linkedNodeExists(ctx context.Context, nodeName string) (bool, error) {
	node := &corev1.Node{}
	if err := r.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get node %q: %w", nodeName, err)
	}

	if !isStaticNode(node) {
		return false, nil
	}

	return true, nil
}
