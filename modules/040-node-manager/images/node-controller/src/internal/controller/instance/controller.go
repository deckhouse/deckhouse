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

package instance_controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/instance/capi"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	instancepkg "github.com/deckhouse/node-controller/internal/controller/instance/instance"
	"github.com/deckhouse/node-controller/internal/controller/instance/mcm"
	instancenode "github.com/deckhouse/node-controller/internal/controller/instance/node"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

const (
	instanceRequeueInterval         = time.Minute
	instanceDeletionRequeueInterval = 5 * time.Second
)

type InstanceController struct {
	dynctrl.Base

	machineFactory machine.MachineFactory
	capiService    *capi.CAPIMachineService
	mcmService     *mcm.MCMMachineService
	instanceSvc    *instancepkg.InstanceService
}

var (
	_ dynctrl.Reconciler = (*InstanceController)(nil)
	_ dynctrl.NeedsSetup = (*InstanceController)(nil)
)

// Setup is called by dynctrl after DI injection, before SetupWatches.
// It wires services using the injected client.
func (r *InstanceController) Setup(_ ctrl.Manager) error {
	r.machineFactory = machine.NewMachineFactory()
	r.capiService = capi.NewCAPIMachineService(r.Client)
	r.mcmService = mcm.NewMCMMachineService(r.Client)
	r.instanceSvc = instancepkg.NewInstanceService(r.Client)
	return nil
}

func (r *InstanceController) SetupWatches(w dynctrl.Watcher) {
	w.Watches(
		&capiv1beta2.Machine{},
		handler.EnqueueRequestsFromMapFunc(instancecommon.MapObjectNameToInstance),
	)

	w.Watches(
		&mcmv1alpha1.Machine{},
		handler.EnqueueRequestsFromMapFunc(instancecommon.MapObjectNameToInstance),
	)

	w.Watches(
		&corev1.Node{},
		handler.EnqueueRequestsFromMapFunc(instancecommon.MapObjectNameToInstance),
		builder.WithPredicates(instancecommon.StaticNodeEventPredicate()),
	)
}

func (r *InstanceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("instance", req.Name)
	logger.Info("reconcile triggered")
	logger.V(4).Info("tick", "op", "instance.reconcile.start")

	instance := &deckhousev1alpha2.Instance{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return r.reconcileCreateFromSource(ctx, req.Name)
		}
		return ctrl.Result{}, err
	}

	type reconcileStep func(ctx context.Context, instance *deckhousev1alpha2.Instance) (done bool, result ctrl.Result, err error)

	for _, step := range []reconcileStep{
		r.instanceSvc.ReconcileHeartbeat,
		r.instanceSvc.ReconcileBashibleStatus,
		r.reconcileDeletion,
		r.instanceSvc.ReconcileEnsureFinalizer,
		r.reconcileMachineStatus,
		r.instanceSvc.ReconcileSourceExistence,
	} {
		done, result, err := step(ctx, instance)
		if err != nil {
			if apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		if done {
			if result != (ctrl.Result{}) {
				logger.V(1).Info("instance reconcile step returned early")
				return result, nil
			}
			break
		}
	}

	logger.V(1).Info("instance reconciled")
	return ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

func (r *InstanceController) reconcileCreateFromSource(ctx context.Context, name string) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("instance", name)
	ns := types.NamespacedName{Namespace: machine.MachineNamespace, Name: name}

	found, err := r.capiService.EnsureInstanceFromMachine(ctx, ns)
	if err != nil {
		return ctrl.Result{}, err
	}
	if found {
		logger.Info("instance ensured from capi machine")
		return ctrl.Result{}, nil
	}

	found, err = r.mcmService.EnsureInstanceFromMachine(ctx, ns)
	if err != nil {
		return ctrl.Result{}, err
	}
	if found {
		logger.Info("instance ensured from mcm machine")
		return ctrl.Result{}, nil
	}

	_, err = instancenode.ReconcileNode(ctx, r.Client, name)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("no source found for instance, nothing to create")
	return ctrl.Result{}, nil
}

func (r *InstanceController) reconcileDeletion(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	isDeleting := instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero()
	if !isDeleting {
		return false, ctrl.Result{}, nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.reconcile.deletion")

	fastRequeue, err := r.instanceSvc.ReconcileFinalization(ctx, instance)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if fastRequeue {
		return true, ctrl.Result{RequeueAfter: instanceDeletionRequeueInterval}, nil
	}

	return true, ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

func (r *InstanceController) reconcileMachineStatus(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return false, ctrl.Result{}, nil
	}

	machineObj, err := r.machineFactory.NewMachineFromRef(ctx, r.Client, ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, ctrl.Result{}, nil
		}
		return false, ctrl.Result{}, err
	}

	status := machineObj.GetStatus()
	if err := instancecommon.SyncInstanceStatus(ctx, r.Client, instance, status); err != nil {
		return false, ctrl.Result{}, err
	}

	return false, ctrl.Result{}, nil
}
