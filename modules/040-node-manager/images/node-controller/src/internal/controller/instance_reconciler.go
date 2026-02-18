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

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// InstanceReconciler owns only Instance reconciliation flow.
// Machine and Node status flows are handled by dedicated controllers.
type InstanceReconciler struct {
	client.Client
}

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

	if err := r.reconcileInstance(ctx, instance); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	log.V(1).Info("instance reconciled")
	return ctrl.Result{}, nil
}

func (r *InstanceReconciler) reconcileInstance(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	_ = ctx
	_ = instance

	// TODO
	return nil
}
