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

package cpnplanner

import (
	"k8s.io/apimachinery/pkg/api/equality"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
)

const maxTerminalOperationsPerComponent = 5

type Plan struct {
	Status *controlplanev1alpha1.ControlPlaneNodeStatus
	Create []PlannedOperation
	Delete []*controlplanev1alpha1.ControlPlaneOperation
}

type PlannedOperation struct {
	Op           *controlplanev1alpha1.ControlPlaneOperation
	HasDuplicate func(active *controlplanev1alpha1.ControlPlaneOperation) bool
}

type OperationBuilder interface {
	Targets(s componentState) []TargetOperation
}

type TargetOperation struct {
	HasDuplicate func(active *controlplanev1alpha1.ControlPlaneOperation) bool
	Build        func(node operations.NodeRef) *controlplanev1alpha1.ControlPlaneOperation
}

func ComputePlan(cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation, builder OperationBuilder) Plan {
	status := ComputeStatusReport(cpn, current)
	var p Plan
	if !equality.Semantic.DeepEqual(cpn.Status, status) {
		p.Status = &status
	}
	if IsMaintenanceMode(cpn) {
		return p
	}
	node := nodeRef(cpn)
	for _, s := range computeComponentStates(cpn) {
		for _, t := range builder.Targets(s) {
			if operations.HasActiveOperation(current, s.component, t.HasDuplicate) {
				continue
			}
			p.Create = append(p.Create, PlannedOperation{
				Op:           t.Build(node),
				HasDuplicate: t.HasDuplicate,
			})
		}
	}
	p.Delete = operations.ComputeOperationsToRotate(current, maxTerminalOperationsPerComponent)
	return p
}

func OwnedOperations(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) []controlplanev1alpha1.ControlPlaneOperation {
	return operations.FilterOwnedBy(ops, nodeRef(cpn))
}

func IsMaintenanceMode(cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	_, ok := cpn.Labels[constants.MaintenanceModeLabelKey]
	return ok
}

func nodeRef(cpn *controlplanev1alpha1.ControlPlaneNode) operations.NodeRef {
	return operations.NodeRef{
		Namespace: cpn.Namespace,
		Name:      cpn.Name,
		Type:      cpn.Labels[constants.ControlPlaneTypeLabelKey],
		UID:       cpn.UID,
	}
}
