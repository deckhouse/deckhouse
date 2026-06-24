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

package controller

import (
	"context"
	"fmt"

	deckhousev1alpha1 "integrity-controller/api/deckhouse.io/v1alpha1"
	//nolint:goimports
	//nolint:gci
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"integrity-containerd-configurator/internal/configwriter"
)

// Reconciler watches ContainerdIntegrityPolicy resources and writes containerd config on the node.
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Writer *configwriter.Writer
}

// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=deckhouse.io,resources=containerdintegritypolicies/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	policy := &deckhousev1alpha1.ContainerdIntegrityPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("get ContainerdIntegrityPolicy: %w", err)
		}
	}

	policyList := &deckhousev1alpha1.ContainerdIntegrityPolicyList{}
	if err := r.List(ctx, policyList); err != nil {
		return reconcile.Result{}, fmt.Errorf("list ContainerdIntegrityPolicies: %w", err)
	}

	desired, err := configwriter.AggregatePolicies(policyList.Items)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("aggregate policies: %w", err)
	}

	if err := r.Writer.Apply(desired); err != nil {
		return reconcile.Result{}, fmt.Errorf("apply containerd integrity config: %w", err)
	}

	if desired == nil {
		logger.Info("Removed containerd integrity config, no policies found")
	} else {
		logger.Info("Updated containerd integrity config", "namespaces", desired.Namespaces, "caCerts", len(desired.CACerts))
	}

	return reconcile.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deckhousev1alpha1.ContainerdIntegrityPolicy{}).
		Complete(r)
}
