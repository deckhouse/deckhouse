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

	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	machinecontroller "github.com/deckhouse/node-controller/internal/controller/machine"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MCMMachineReconciler struct {
	client.Client
}

func SetupMCMMachineController(mgr ctrl.Manager) error {
	if err := (&MCMMachineReconciler{
		Client: mgr.GetClient(),
	}).
		SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to setup mcm machine reconciler: %w", err)
	}

	return nil
}

func (r *MCMMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("mcm-machine-controller").
		For(&mcmv1alpha1.Machine{}).
		Complete(r)
}

func (r *MCMMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("mcmMachine", req.NamespacedName.String())
	factory := machinecontroller.NewMachineFactory()

	key := req.NamespacedName
	machine := &mcmv1alpha1.Machine{}
	if err := r.Get(ctx, key, machine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	machineAdapter, err := factory.NewMachine(machine)
	if err != nil {
		return ctrl.Result{}, err
	}
	status := machineAdapter.GetStatus()
	nodeGroup := machineAdapter.GetNodeGroup()
	log.Info("MCMMachineReconciler", "status", status, "nodeGroup", nodeGroup)

	return ctrl.Result{}, nil
}
