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

package capi

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	instancecommon "github.com/deckhouse/node-controller/internal/controller/instance/common"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

// CAPIMachineService contains the reconcile logic for linking a CAPI Machine to an Instance.
// It is stateless and receives a client on each call.
type CAPIMachineService struct {
	machineFactory machine.MachineFactory
}

// NewCAPIMachineService creates a CAPIMachineService with the default machine factory.
func NewCAPIMachineService() *CAPIMachineService {
	return &CAPIMachineService{
		machineFactory: machine.NewMachineFactory(),
	}
}

// ReconcileMachine fetches the CAPI Machine by name and reconciles the linked Instance.
// Returns (deleted, error): deleted=true means Instance was deleted because Machine is gone.
func (s *CAPIMachineService) ReconcileMachine(ctx context.Context, c client.Client, name types.NamespacedName) (bool, error) {
	logger := log.FromContext(ctx).WithValues("capiMachine", name.String())
	logger.V(4).Info("tick", "op", "capi.reconcile.start")

	capiMachine := &capiv1beta2.Machine{}
	if err := c.Get(ctx, name, capiMachine); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return false, err
		}
		// Machine gone — delete linked Instance
		deleted, err := deleteInstanceIfExists(ctx, c, name.Name)
		if err != nil {
			return false, err
		}
		logger.V(1).Info("machine not found, linked instance delete handled", "instance", name.Name, "deleted", deleted)
		return deleted, nil
	}

	machineObj, err := s.machineFactory.NewMachine(capiMachine)
	if err != nil {
		return false, fmt.Errorf("build reconcile data for capi machine %q: %w", capiMachine.Name, err)
	}

	data := capiMachineReconcileData{
		capiMachine:   capiMachine,
		instanceName:  machineObj.GetName(),
		nodeName:      machineObj.GetNodeName(),
		machineRef:    machineObj.GetMachineRef(),
		machineStatus: machineObj.GetStatus(),
		nodeGroup:     machineObj.GetNodeGroup(),
	}

	if err := reconcileLinkedInstance(ctx, c, data); err != nil {
		return false, err
	}

	logger.V(1).Info("reconcile complete", "status", data.machineStatus, "nodeGroup", data.nodeGroup)
	return false, nil
}

func (s *CAPIMachineService) EnsureInstanceFromMachine(
	ctx context.Context,
	c client.Client,
	name types.NamespacedName,
) (bool, error) {
	capiMachine := &capiv1beta2.Machine{}
	if err := c.Get(ctx, name, capiMachine); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, err
	}

	machineObj, err := s.machineFactory.NewMachine(capiMachine)
	if err != nil {
		return false, fmt.Errorf("build machine for capi %q: %w", capiMachine.Name, err)
	}

	spec := deckhousev1alpha2.InstanceSpec{}
	if nodeName := machineObj.GetNodeName(); nodeName != "" {
		spec.NodeRef = deckhousev1alpha2.NodeRef{Name: nodeName}
	}
	if ref := machineObj.GetMachineRef(); ref != nil {
		refCopy := *ref
		spec.MachineRef = &refCopy
	}
	if _, err := instancecommon.EnsureInstanceExists(ctx, c, machineObj.GetName(), spec); err != nil {
		return false, err
	}
	return true, nil
}

// capiMachineReconcileData holds computed data for one reconcile pass.
type capiMachineReconcileData struct {
	capiMachine   *capiv1beta2.Machine
	instanceName  string
	nodeName      string
	machineRef    *deckhousev1alpha2.MachineRef
	machineStatus machine.MachineStatus
	nodeGroup     string
}

func reconcileLinkedInstance(ctx context.Context, c client.Client, data capiMachineReconcileData) error {
	logger := log.FromContext(ctx)
	logger.V(4).Info("tick", "op", "capi.instance.reconcile")

	instance, err := ensureInstanceExists(ctx, c, data.instanceName, data.nodeName, data.machineRef)
	if err != nil {
		return err
	}

	instance, specUpdated, err := syncInstanceSpec(ctx, c, instance, data.nodeName, data.machineRef)
	if err != nil {
		return err
	}

	machineDeleteRequested, err := ensureMachineDeletionForDeletingInstance(ctx, c, data.capiMachine, instance)
	if err != nil {
		return err
	}

	if err := instancecommon.SyncInstanceStatus(ctx, c, instance, data.machineStatus); err != nil {
		return err
	}

	logger.V(1).Info(
		"linked instance reconciled",
		"instance", instance.Name,
		"specUpdated", specUpdated,
		"machineDeleteRequested", machineDeleteRequested,
	)
	return nil
}

func ensureInstanceExists(
	ctx context.Context,
	c client.Client,
	name string,
	nodeName string,
	machineRef *deckhousev1alpha2.MachineRef,
) (*deckhousev1alpha2.Instance, error) {
	spec := deckhousev1alpha2.InstanceSpec{}
	if nodeName != "" {
		spec.NodeRef = deckhousev1alpha2.NodeRef{Name: nodeName}
	}
	if machineRef != nil {
		refCopy := *machineRef
		spec.MachineRef = &refCopy
	}

	return instancecommon.EnsureInstanceExists(ctx, c, name, spec)
}

func syncInstanceSpec(
	ctx context.Context,
	c client.Client,
	instance *deckhousev1alpha2.Instance,
	nodeName string,
	machineRef *deckhousev1alpha2.MachineRef,
) (*deckhousev1alpha2.Instance, bool, error) {
	updated := instance.DeepCopy()
	if nodeName != "" {
		updated.Spec.NodeRef = deckhousev1alpha2.NodeRef{Name: nodeName}
	}
	if machineRef == nil {
		updated.Spec.MachineRef = nil
	} else {
		refCopy := *machineRef
		updated.Spec.MachineRef = &refCopy
	}

	if apiequality.Semantic.DeepEqual(instance.Spec, updated.Spec) {
		return instance, false, nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "capi.instance.spec.patch")

	if err := c.Patch(ctx, updated, client.MergeFrom(instance)); err != nil {
		return nil, false, fmt.Errorf("patch instance %q spec: %w", instance.Name, err)
	}
	return updated, true, nil
}

func ensureMachineDeletionForDeletingInstance(
	ctx context.Context,
	c client.Client,
	capiMachine *capiv1beta2.Machine,
	instance *deckhousev1alpha2.Instance,
) (bool, error) {
	if !isBeingDeleted(instance.DeletionTimestamp) || isBeingDeleted(capiMachine.DeletionTimestamp) {
		return false, nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "capi.machine.delete.request")

	if err := c.Delete(ctx, capiMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("delete capi machine %q for deleting instance %q: %w", capiMachine.Name, instance.Name, err)
	}
	return true, nil
}

func deleteInstanceIfExists(ctx context.Context, c client.Client, name string) (bool, error) {
	log.FromContext(ctx).V(4).Info("tick", "op", "capi.instance.delete")
	instance := &deckhousev1alpha2.Instance{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if err := c.Delete(ctx, instance); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return false, nil
		}
		return false, fmt.Errorf("delete instance %q: %w", name, err)
	}
	log.FromContext(ctx).V(1).Info(
		"instance deleted",
		"instance", name,
		"deletedBy", "capi-machine-controller",
		"reason", "linked-machine-not-found",
	)

	return true, nil
}

func isBeingDeleted(ts *metav1.Time) bool {
	return ts != nil && !ts.IsZero()
}
