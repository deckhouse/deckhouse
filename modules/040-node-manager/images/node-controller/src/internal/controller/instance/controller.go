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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/instance/capi"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	instancepkg "github.com/deckhouse/node-controller/internal/controller/instance/instance"
	"github.com/deckhouse/node-controller/internal/controller/instance/mcm"
	instancenode "github.com/deckhouse/node-controller/internal/controller/instance/node"
	"github.com/deckhouse/node-controller/internal/register"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

func init() {
	register.RegisterController(
		register.InstanceControllers,
		&deckhousev1alpha2.Instance{},
		&InstanceController{},
	)
}

const instanceRequeueInterval = time.Minute

type InstanceController struct {
	dynctrl.Base

	initOnce       sync.Once
	machineFactory machine.MachineFactory
	capiService    *capi.CAPIMachineService
	mcmService     *mcm.MCMMachineService
}

var _ dynctrl.Reconciler = (*InstanceController)(nil)

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

func (r *InstanceController) init() {
	r.initOnce.Do(func() {
		r.machineFactory = machine.NewMachineFactory()
		r.capiService = capi.NewCAPIMachineService()
		r.mcmService = mcm.NewMCMMachineService()
	})
}

func (r *InstanceController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.init()

	log := ctrl.LoggerFrom(ctx).WithValues("instance", req.Name)
	log.V(4).Info("tick", "op", "instance.reconcile.start")

	instance := &deckhousev1alpha2.Instance{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			// Instance not found — try to create it from a linked source (Machine or Node).
			return r.reconcileCreateFromSource(ctx, req.Name)
		}
		return ctrl.Result{}, err
	}

	instanceSvc := instancepkg.NewInstanceService(r.Client)

	type reconcileStep func(ctx context.Context, instance *deckhousev1alpha2.Instance) (done bool, result ctrl.Result, err error)

	for _, step := range []reconcileStep{
		// refresh bashible heartbeat condition by timeout rules
		instanceSvc.ReconcileHeartbeat,
		// derive bashible status and message from current conditions
		instanceSvc.ReconcileBashibleStatus,
		// handle deleting instance and run finalization flow
		r.reconcileDeletion,
		// ensure controller finalizer exists on active object
		instanceSvc.ReconcileEnsureFinalizer,
		// sync Instance status (Phase, MachineStatus, MachineReady) from linked Machine
		r.reconcileMachineStatus,
		// remove instance when both linked sources are confirmed missing
		instanceSvc.ReconcileSourceExistence,
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
				log.V(1).Info("instance reconcile step returned early")
				return result, nil
			}
			break
		}
	}

	log.V(1).Info("instance reconciled")
	return ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

func (r *InstanceController) reconcileCreateFromSource(ctx context.Context, name string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("instance", name)
	ns := types.NamespacedName{Namespace: machine.MachineNamespace, Name: name}

	// Try CAPI Machine first.
	_, err := r.capiService.ReconcileMachine(ctx, r.Client, ns)
	if err == nil {
		log.V(1).Info("instance ensured from capi machine")
		return ctrl.Result{}, nil
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Try MCM Machine.
	_, err = r.mcmService.ReconcileMachine(ctx, r.Client, ns)
	if err == nil {
		log.V(1).Info("instance ensured from mcm machine")
		return ctrl.Result{}, nil
	}
	if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// Try static Node.
	_, err = instancenode.ReconcileNode(ctx, r.Client, name)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.V(4).Info("no source found for instance, nothing to create")
	return ctrl.Result{}, nil
}

// reconcileDeletion handles the deletion flow for an Instance.
func (r *InstanceController) reconcileDeletion(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
) (bool, ctrl.Result, error) {
	isDeleting := instance.DeletionTimestamp != nil && !instance.DeletionTimestamp.IsZero()
	if !isDeleting {
		return false, ctrl.Result{}, nil
	}
	ctrl.LoggerFrom(ctx).V(4).Info("tick", "op", "instance.reconcile.deletion")

	instanceSvc := instancepkg.NewInstanceService(r.Client)
	fastRequeue, err := instanceSvc.ReconcileFinalization(ctx, instance)
	if err != nil {
		return false, ctrl.Result{}, err
	}
	if fastRequeue {
		return true, ctrl.Result{RequeueAfter: time.Second}, nil
	}

	return true, ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

// reconcileMachineStatus syncs Instance.Status (Phase, MachineStatus, MachineReady condition)
// from the linked Machine using the machine factory.
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
			// Machine is gone — source existence step will handle cleanup
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
