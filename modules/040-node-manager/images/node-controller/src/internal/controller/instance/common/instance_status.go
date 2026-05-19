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

package common

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	deckhousev1alpha2 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
	"github.com/deckhouse/node-controller/internal/controller/instance/common/machine"
)

const (
	InstanceMachineStatusFieldOwner = "node-controller-instancestatus"
	InstancePhaseFieldOwner         = "node-controller-instancephase"
)

func SyncInstanceStatus(ctx context.Context, c client.Client, instance *deckhousev1alpha2.Instance, machineStatus machine.MachineStatus) error {
	if machineStatus.MachineReadyCondition == nil {
		return fmt.Errorf("build desired MachineReady condition for instance %q: condition is nil", instance.Name)
	}

	currentCondition, hasCurrent := GetInstanceConditionByType(
		instance.Status.Conditions,
		deckhousev1alpha2.InstanceConditionTypeMachineReady,
	)
	desiredMachineReadyCondition := *machineStatus.MachineReadyCondition

	// Keep LastTransitionTime stable while condition semantics do not change.
	if hasCurrent &&
		currentCondition.Type == desiredMachineReadyCondition.Type &&
		currentCondition.Status == desiredMachineReadyCondition.Status {
		desiredMachineReadyCondition.LastTransitionTime = currentCondition.LastTransitionTime
	}

	conditionChanged := !hasCurrent ||
		!ConditionEqualExceptLastTransitionTime(currentCondition, desiredMachineReadyCondition)

	needsPatch := instance.Status.Phase != machineStatus.Phase ||
		instance.Status.MachineStatus != string(machineStatus.Status) ||
		conditionChanged

	if !needsPatch {
		return nil
	}
	log.FromContext(ctx).V(4).Info("tick", "op", "instance.machine_status.patch")

	if err := ApplyInstanceMachineStatus(
		ctx,
		c,
		instance.Name,
		machineStatus.Phase,
		machineStatus.Status,
		desiredMachineReadyCondition,
	); err != nil {
		return err
	}

	instance.Status.Phase = machineStatus.Phase
	instance.Status.MachineStatus = string(machineStatus.Status)
	instance.Status.Conditions = upsertInstanceCondition(instance.Status.Conditions, desiredMachineReadyCondition)
	return nil
}

func ApplyInstanceMachineStatus(ctx context.Context, c client.Client, instanceName string, phase deckhousev1alpha2.InstancePhase, status machine.Status, machineReadyCondition deckhousev1alpha2.InstanceCondition) error {
	applyObj := InstanceApplyObject(instanceName)
	applyObj.Status = deckhousev1alpha2.InstanceStatus{
		Phase:         phase,
		MachineStatus: string(status),
		Conditions:    []deckhousev1alpha2.InstanceCondition{machineReadyCondition},
	}

	if err := c.Status().Patch(
		ctx,
		applyObj,
		client.Apply,
		client.FieldOwner(InstanceMachineStatusFieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply instance machine status for %q: %w", instanceName, err)
	}

	return nil
}

func ApplyInstancePhase(ctx context.Context, c client.Client, instanceName string, phase deckhousev1alpha2.InstancePhase) error {
	applyObj := InstanceApplyObject(instanceName)
	applyObj.Status = deckhousev1alpha2.InstanceStatus{Phase: phase}

	if err := c.Status().Patch(
		ctx,
		applyObj,
		client.Apply,
		client.FieldOwner(InstancePhaseFieldOwner),
		client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("apply instance %q phase: %w", instanceName, err)
	}

	return nil
}

func InstanceApplyObject(name string) *deckhousev1alpha2.Instance {
	return &deckhousev1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deckhousev1alpha2.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func ConditionEqualExceptLastTransitionTime(left deckhousev1alpha2.InstanceCondition, right deckhousev1alpha2.InstanceCondition) bool {
	left.LastTransitionTime = nil
	right.LastTransitionTime = nil

	return apiequality.Semantic.DeepEqual(left, right)
}

func upsertInstanceCondition(conditions []deckhousev1alpha2.InstanceCondition, condition deckhousev1alpha2.InstanceCondition) []deckhousev1alpha2.InstanceCondition {
	for i := range conditions {
		if conditions[i].Type == condition.Type {
			conditions[i] = condition
			return conditions
		}
	}

	return append(conditions, condition)
}
