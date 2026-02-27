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

package instance

import (
	"context"
	"fmt"
	"time"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/machine"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstanceReconciler struct {
	client.Client
	machineFactory        machine.MachineFactory
	bashibleStatusFactory BashibleStatusFactory
	messageFactory        MessageFactory
}

const instanceControllerFinalizer = "node-manager.hooks.deckhouse.io/instance-controller"
const instanceRequeueInterval = time.Minute

func SetupInstanceController(mgr ctrl.Manager) error {
	if err := (&InstanceReconciler{
		Client:                mgr.GetClient(),
		machineFactory:        machine.NewMachineFactory(),
		bashibleStatusFactory: NewBashibleStatusFactory(),
		messageFactory:        NewMessageFactory(),
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
	if r.messageFactory == nil {
		return fmt.Errorf("messageFactory is required")
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
