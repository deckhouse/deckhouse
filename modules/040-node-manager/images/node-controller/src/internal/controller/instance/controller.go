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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
	instancepkg "github.com/deckhouse/node-controller/internal/controller/instance/instance"
	instancenode "github.com/deckhouse/node-controller/internal/controller/instance/node"
	"github.com/deckhouse/node-controller/internal/register/dynctrl"
)

const instanceRequeueInterval = time.Minute

type InstanceController struct {
	dynctrl.Base

	machineFactory machine.MachineFactory
	instanceSvc    *instancepkg.InstanceService
}

func (r *InstanceController) Setup(_ ctrl.Manager) error {
	r.machineFactory = machine.NewMachineFactory()
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

	for _, s := range []reconcileStep{
		r.reconcileDeletion,
		nonTerminalStep(r.instanceSvc.EnsureInstanceFinalizer),
		nonTerminalStep(r.reconcileMachineRef),
		r.instanceSvc.ReconcileSourceExistence,
		nonTerminalStep(r.instanceSvc.ReconcileBashibleHeartbeat),
		nonTerminalStep(r.instanceSvc.ReconcileBashibleStatus),
		nonTerminalStep(r.reconcileMachineStatus),
	} {
		done, err := s(ctx, instance)
		if err != nil {
			return resultFromErr(err)
		}
		if done {
			break
		}
	}

	logger.V(1).Info("instance reconciled")
	return ctrl.Result{RequeueAfter: instanceRequeueInterval}, nil
}

type reconcileStep func(context.Context, *deckhousev1alpha2.Instance) (bool, error)

func nonTerminalStep(fn func(context.Context, *deckhousev1alpha2.Instance) error) reconcileStep {
	return func(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
		return false, fn(ctx, instance)
	}
}

func (r *InstanceController) reconcileDeletion(ctx context.Context, instance *deckhousev1alpha2.Instance) (bool, error) {
	if instance.DeletionTimestamp == nil || instance.DeletionTimestamp.IsZero() {
		return false, nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.reconcile.deletion")
	machineGone, err := r.instanceSvc.ReconcileFinalization(ctx, instance)
	if err != nil || machineGone {
		return true, err
	}
	if err := r.reconcileMachineStatus(ctx, instance); err != nil {
		return true, err
	}
	return true, r.instanceSvc.ReconcileBashibleStatus(ctx, instance)
}

func resultFromErr(err error) (ctrl.Result, error) {
	if apierrors.IsConflict(err) {
		return ctrl.Result{Requeue: true}, nil
	}
	return ctrl.Result{}, err
}

func (r *InstanceController) reconcileCreateFromSource(ctx context.Context, name string) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("instance", name)

	m, found, err := r.findMachineForInstance(ctx, name)
	if err != nil {
		return ctrl.Result{}, err
	}
	if found {
		if err := r.createInstanceFromMachine(ctx, m); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("instance ensured from machine")
		return ctrl.Result{}, nil
	}

	if _, err := instancenode.ReconcileNode(ctx, r.Client, name); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("no source found for instance, nothing to create")
	return ctrl.Result{}, nil
}

func (r *InstanceController) reconcileMachineRef(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	if instance.Spec.MachineRef != nil && instance.Spec.MachineRef.Name != "" {
		return nil
	}

	m, found, err := r.findMachineForInstance(ctx, instance.Name)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	ref := m.GetMachineRef()
	if ref == nil {
		return nil
	}
	if err := r.patchInstanceMachineRef(ctx, instance, ref); err != nil {
		return err
	}
	log.FromContext(ctx).Info("instance machine ref self-healed", "instance", instance.Name, "ref", ref.Name)
	return nil
}

func (r *InstanceController) reconcileMachineStatus(ctx context.Context, instance *deckhousev1alpha2.Instance) error {
	ref := instance.Spec.MachineRef
	if ref == nil || ref.Name == "" {
		return nil
	}

	machineObj, err := r.machineFactory.NewMachineFromRef(ctx, r.Client, ref)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return instancecommon.SyncInstanceStatus(ctx, r.Client, instance, machineObj.GetStatus())
}

func (r *InstanceController) findMachineForInstance(ctx context.Context, name string) (machine.Machine, bool, error) {
	ns := types.NamespacedName{Namespace: machine.MachineNamespace, Name: name}

	capiObj := &capiv1beta2.Machine{}
	if err := r.Client.Get(ctx, ns, capiObj); err == nil {
		m, err := r.machineFactory.NewMachine(capiObj)
		if err != nil {
			return nil, false, fmt.Errorf("wrap capi machine %q: %w", name, err)
		}
		return m, true, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, false, fmt.Errorf("get capi machine %q: %w", name, err)
	}

	mcmObj := &mcmv1alpha1.Machine{}
	if err := r.Client.Get(ctx, ns, mcmObj); err == nil {
		m, err := r.machineFactory.NewMachine(mcmObj)
		if err != nil {
			return nil, false, fmt.Errorf("wrap mcm machine %q: %w", name, err)
		}
		return m, true, nil
	} else if !apierrors.IsNotFound(err) {
		return nil, false, fmt.Errorf("get mcm machine %q: %w", name, err)
	}

	return nil, false, nil
}

func (r *InstanceController) createInstanceFromMachine(ctx context.Context, m machine.Machine) error {
	spec := deckhousev1alpha2.InstanceSpec{}
	if nodeName := m.GetNodeName(); nodeName != "" {
		spec.NodeRef = deckhousev1alpha2.NodeRef{Name: nodeName}
	}
	if ref := m.GetMachineRef(); ref != nil {
		refCopy := *ref
		spec.MachineRef = &refCopy
	}
	if _, err := instancecommon.EnsureInstanceExists(ctx, r.Client, m.GetName(), spec); err != nil {
		return fmt.Errorf("ensure instance for machine %q: %w", m.GetName(), err)
	}
	return nil
}

func (r *InstanceController) patchInstanceMachineRef(
	ctx context.Context,
	instance *deckhousev1alpha2.Instance,
	ref *deckhousev1alpha2.MachineRef,
) error {
	patch := client.MergeFrom(instance.DeepCopy())
	refCopy := *ref
	instance.Spec.MachineRef = &refCopy
	if err := r.Client.Patch(ctx, instance, patch); err != nil {
		return fmt.Errorf("patch instance %q machine ref: %w", instance.Name, err)
	}
	return nil
}
