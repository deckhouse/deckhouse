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

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
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
}

const instanceControllerFinalizer = "node-manager.hooks.deckhouse.io/instance-controller"

func SetupInstanceController(mgr ctrl.Manager) error {
	if err := (&InstanceReconciler{
		Client: mgr.GetClient(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup instance reconciler: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
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
	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstance(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	isDeleting := instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero()
	if isDeleting {
		return r.reconcileInstanceDeletion(ctx, instance)
	}

	return false, r.ensureInstanceFinalizer(ctx, instance)
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

	namespace := ref.Namespace
	if namespace == "" {
		namespace = MachineNamespace
	}

	switch ref.APIVersion {
	case "", capiv1beta2.GroupVersion.String():
		return r.ensureCAPIMachineDeleted(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name})
	case mcmv1alpha1.SchemeGroupVersion.String():
		return r.ensureMCMMachineDeleted(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name})
	default:
		return true, nil
	}
}

func (r *InstanceReconciler) ensureCAPIMachineDeleted(ctx context.Context, key types.NamespacedName) (bool, error) {
	machine := &capiv1beta2.Machine{}
	if err := r.Get(ctx, key, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("get capi machine %q: %w", key.String(), err)
	}

	if !machine.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if err := r.Delete(ctx, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("delete capi machine %q: %w", key.String(), err)
	}

	return false, nil
}

func (r *InstanceReconciler) ensureMCMMachineDeleted(ctx context.Context, key types.NamespacedName) (bool, error) {
	machine := &mcmv1alpha1.Machine{}
	if err := r.Get(ctx, key, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("get mcm machine %q: %w", key.String(), err)
	}

	if !machine.DeletionTimestamp.IsZero() {
		return false, nil
	}

	if err := r.Delete(ctx, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("delete mcm machine %q: %w", key.String(), err)
	}

	return false, nil
}
