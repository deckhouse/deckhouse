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
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpnplanner"
)

const ControllerName = constants.VirtualControlPlaneNodeController

func BuildController(mgr manager.Manager) error {
	r := &reconciler{
		client: mgr.GetClient(),
		// apiReader is an uncached reader used to confirm, right before creating an operation, that the previous reconcile of the same node did not already create it.
		apiReader:        mgr.GetAPIReader(),
		scheme:           mgr.GetScheme(),
		operationBuilder: cpnplanner.VirtualOperationBuilder{},
	}

	cpnPreds, err := controlPlaneNodePredicates()
	if err != nil {
		return fmt.Errorf("build ControlPlaneNode predicates: %w", err)
	}
	cpoPreds, err := controlPlaneOperationPredicates()
	if err != nil {
		return fmt.Errorf("build ControlPlaneOperation predicates: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: 10,
		}).
		Named(ControllerName).
		For(&controlplanev1alpha1.ControlPlaneNode{}, builder.WithPredicates(cpnPreds...)).
		Owns(&controlplanev1alpha1.ControlPlaneOperation{}, builder.WithPredicates(cpoPreds...)).
		Complete(r)
}
