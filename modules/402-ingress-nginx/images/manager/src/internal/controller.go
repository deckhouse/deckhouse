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

package internal

import (
	"context"
	"manager/src/internal/helper"
	"os"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	v1 "manager/src/api/v1"
)

type IngressNginxController struct {
	client.Client
	Pods      *helper.PodService
	Workloads *helper.WorkloadService
}

// Reconcile is the main loop that moves current state closer to desired state.
func (r *IngressNginxController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var ic v1.IngressNginxController
	if err := r.Get(ctx, req.NamespacedName, &ic); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 1. Finalizer check
	result, stop, err := r.reconcileFinalizer(ctx, &ic)
	if err != nil || stop || result.Requeue || result.RequeueAfter > 0 {
		return result, err
	}

	// 2. Migrate check
	result, err = r.checkMigration(ctx, &ic)
	if err != nil || result.Requeue || result.RequeueAfter > 0 {
		return result, err
	}

	// 3. Rollout check
	result, err = r.checkRollout(ctx, &ic)
	if err != nil || result.Requeue || result.RequeueAfter > 0 {
		return result, err
	}

	// 4. Handle ingressnginx

	return ctrl.Result{}, nil
}

func SetupController(manager ctrl.Manager, logger logr.Logger) {
	err := ctrl.
		NewControllerManagedBy(manager).
		For(&v1.IngressNginxController{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		Complete(&IngressNginxController{
			Client:    manager.GetClient(),
			Pods:      helper.NewPodService(manager.GetClient()),
			Workloads: helper.NewWorkloadService(manager.GetClient()),
		})
	if err != nil {
		logger.Error(err, "could not create controller")
		os.Exit(1)
	}
}

// checkMigration
func (r *IngressNginxController) checkMigration(ctx context.Context, ic *v1.IngressNginxController) (ctrl.Result, error) {
	r.ensureServices()

	switch ic.Spec.Inlet {
	case v1.InletHostPort, v1.InletHostPortWithSSLPassthrough, v1.InletHostPortWithProxyProtocol:
		return r.runMigration(ctx, ic, r.MigrateHostInlet)
	case v1.InletLoadBalancer, v1.InletLoadBalancerWithSSLPassthrough, v1.InletLoadBalancerWithProxyProtocol:
		return r.runMigration(ctx, ic, r.MigrateLoadBalancer)
	case v1.InletHostWithFailover:
		return r.runMigration(ctx, ic, r.MigrateHostWithFailover)
	}

	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

func (r *IngressNginxController) runMigration(
	ctx context.Context,
	ic *v1.IngressNginxController,
	migrate func(context.Context, *v1.IngressNginxController) (ctrl.Result, error),
) (ctrl.Result, error) {
	state := helper.GetMigrationState(ic)
	if state == helper.MigrationStateMigrated {
		return ctrl.Result{}, nil
	}

	result, err := migrate(ctx, ic)
	if err != nil {
		return ctrl.Result{}, err
	}

	if result.Requeue || result.RequeueAfter > 0 {
		if state == "" {
			if err := helper.PatchMigrationState(ctx, r.Client, ic, helper.MigrationStateRunning); err != nil {
				return ctrl.Result{}, err
			}
		}

		return result, nil
	}

	if err := helper.PatchMigrationState(ctx, r.Client, ic, helper.MigrationStateMigrated); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *IngressNginxController) checkRollout(ctx context.Context, ic *v1.IngressNginxController) (ctrl.Result, error) {
	switch ic.Spec.Inlet {
	case v1.InletHostWithFailover:
		return r.rolloutHostWithFailover(ctx, ic)
	default:
		return ctrl.Result{}, nil
	}
}

func (r *IngressNginxController) rolloutHostWithFailover(ctx context.Context, ic *v1.IngressNginxController) (ctrl.Result, error) {
	return r.SafeUpdateHostWithFailover(ctx, ic)
}

func (r *IngressNginxController) ensureServices() {
	if r.Pods == nil {
		r.Pods = helper.NewPodService(r.Client)
	}
	if r.Workloads == nil {
		r.Workloads = helper.NewWorkloadService(r.Client)
	}
}
