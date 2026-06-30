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
	"log/slog"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/cpn/cpnplanner"
	"control-plane-manager/internal/cpn/cpnreconcile"
)

func BuildController(mgr manager.Manager) error {
	r := cpnreconcile.New(
		mgr.GetClient(),
		mgr.GetAPIReader(),
		mgr.GetScheme(),
		cpnplanner.VirtualOperationBuilder{},
		nil,
		log.Default().With(slog.String("controller", constants.VirtualControlPlaneNodeController)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{
			MaxConcurrentReconciles: 10,
		}).
		Named(constants.VirtualControlPlaneNodeController).
		For(&controlplanev1alpha1.ControlPlaneNode{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&controlplanev1alpha1.ControlPlaneOperation{}, builder.WithPredicates(cpnreconcile.OperationStatusChangedPredicate())).
		Complete(r)
}
