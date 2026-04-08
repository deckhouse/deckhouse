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

package instance

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
)

func (r *InstanceReconciler) updateInstanceStatus(ctx context.Context, instance *deckhousev1alpha1.Instance, machine *mcmv1alpha1.Machine, ng *deckhousev1.NodeGroup) error {
	patch := client.MergeFrom(instance.DeepCopy())
	changed := false

	machineNodeName := machine.GetLabels()["node"]
	if instance.Status.NodeRef.Name != machineNodeName {
		instance.Status.NodeRef.Name = machineNodeName
		changed = true
	}

	if machine.Status.CurrentStatus.Phase == mcmv1alpha1.MachinePhaseRunning &&
		(instance.Status.BootstrapStatus.LogsEndpoint != "" || instance.Status.BootstrapStatus.Description != "") {
		instance.Status.BootstrapStatus = deckhousev1alpha1.InstanceBootstrapStatus{}
		changed = true
	}

	machinePhase := deckhousev1alpha1.InstancePhase(machine.Status.CurrentStatus.Phase)
	if instance.Status.CurrentStatus.Phase != machinePhase {
		instance.Status.CurrentStatus.Phase = machinePhase
		instance.Status.CurrentStatus.LastUpdateTime = machine.Status.CurrentStatus.LastUpdateTime
		changed = true
	}

	if updateLastOperation(instance, &machine.Status.LastOperation) {
		changed = true
	}

	if ng.Spec.CloudInstances != nil {
		ngClassRef := ng.Spec.CloudInstances.ClassReference
		if instance.Status.ClassReference.Kind != ngClassRef.Kind || instance.Status.ClassReference.Name != ngClassRef.Name {
			instance.Status.ClassReference.Kind = ngClassRef.Kind
			instance.Status.ClassReference.Name = ngClassRef.Name
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if err := r.Client.Status().Patch(ctx, instance, patch); err != nil {
		return fmt.Errorf("patch instance %s status: %w", instance.Name, err)
	}

	return nil
}

func updateLastOperation(instance *deckhousev1alpha1.Instance, machineOp *mcmv1alpha1.MachineLastOperation) bool {
	if machineOp == nil {
		return false
	}

	changed := false
	shouldUpdate := true

	if instance.Status.LastOperation.Description != machineOp.Description {
		if machineOp.Description == "Started Machine creation process" {
			shouldUpdate = false
		} else {
			instance.Status.LastOperation.Description = machineOp.Description
			changed = true
		}
	}

	if !shouldUpdate {
		return changed
	}

	opType := deckhousev1alpha1.InstanceOperationType(machineOp.Type)
	if instance.Status.LastOperation.Type != opType {
		instance.Status.LastOperation.Type = opType
		changed = true
	}

	opState := deckhousev1alpha1.InstanceState(machineOp.State)
	if instance.Status.LastOperation.State != opState {
		instance.Status.LastOperation.State = opState
		changed = true
	}

	if !instance.Status.LastOperation.LastUpdateTime.Equal(&machineOp.LastUpdateTime) {
		instance.Status.LastOperation.LastUpdateTime = machineOp.LastUpdateTime
		changed = true
	}

	return changed
}
