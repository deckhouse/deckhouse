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

// Package approver implements a reusable control-plane operation approval engine.
//
// The pipeline (which components exist, in what stage order, with what concurrency/gating rules)
// is a pluggable Strategy passed into NewApprover; the engine itself is mode-agnostic and reused
// across NormalPipeline and VirtualPipeline (see pipeline.go). Internally, admission decisions are
// made by a Chain of Responsibility of stageGates (see gate.go).
package approver

import (
	"cmp"
	"slices"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

// Nodes describes the node inventory relevant to approval decisions.
type Nodes struct {
	Masters  int
	Arbiters int
}

func (c Nodes) IsZero() bool {
	return c.Masters+c.Arbiters == 0
}

// Approver selects which not-yet-approved operations are safe to approve now, according to an
// injected pipeline strategy. It is immutable and reusable across reconciles.
type Approver struct {
	pipeline []pipelineStage
}

// NewApprover builds an Approver for the given pipeline (see NormalPipeline, VirtualPipeline).
func NewApprover(pipeline []pipelineStage) *Approver {
	return &Approver{pipeline: slices.Clone(pipeline)}
}

// SelectApprovable partitions operations, replays already-approved/in-flight ones through the
// pipeline to reconstruct current occupancy, then returns the subset of not-yet-approved
// operations that are safe to approve now. It is pure: it does not mutate Spec.Approved on any
// operation; the caller is responsible for mutating and persisting the result.
func (a *Approver) SelectApprovable(
	operations []controlplanev1alpha1.ControlPlaneOperation,
	nodes Nodes,
) []controlplanev1alpha1.ControlPlaneOperation {
	approvedOperations, unapprovedOperations := partitionOperationsByApprovalState(operations)
	sortOperationsByPipelineOrder(a.pipeline, approvedOperations)
	sortOperationsByPipelineOrder(a.pipeline, unapprovedOperations)

	gates := buildGateChain(a.pipeline, nodes)
	gates.seedApprovedOperations(approvedOperations)

	var approvable []controlplanev1alpha1.ControlPlaneOperation
	for _, op := range unapprovedOperations {
		if gates.tryAdmit(op) {
			approvable = append(approvable, op)
		}
	}

	return approvable
}

func partitionOperationsByApprovalState(
	operations []controlplanev1alpha1.ControlPlaneOperation,
) (
	approvedOperations []controlplanev1alpha1.ControlPlaneOperation,
	unapprovedOperations []controlplanev1alpha1.ControlPlaneOperation,
) {
	approvedOperations = make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(operations))
	unapprovedOperations = make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(operations))

	for _, operation := range operations {
		if operation.Spec.Approved && !operation.IsTerminal() {
			approvedOperations = append(approvedOperations, operation)
			continue
		}

		if !operation.Spec.Approved {
			unapprovedOperations = append(unapprovedOperations, operation)
		}
	}

	return approvedOperations, unapprovedOperations
}

// sortOperationsByPipelineOrder orders operations by pipelineStageIndex within pipeline.
// Within the same stage, order is stable by resource name.
func sortOperationsByPipelineOrder(pipeline []pipelineStage, operations []controlplanev1alpha1.ControlPlaneOperation) {
	slices.SortFunc(operations, func(a, b controlplanev1alpha1.ControlPlaneOperation) int {
		if c := cmp.Compare(pipelineStageIndex(pipeline, a.Spec.Component), pipelineStageIndex(pipeline, b.Spec.Component)); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
}

// pipelineStageIndex returns the stage index of c within pipeline, or -1 if unknown.
func pipelineStageIndex(pipeline []pipelineStage, c controlplanev1alpha1.OperationComponent) int {
	for i, stage := range pipeline {
		if slices.Contains(stage.components, c) {
			return i
		}
	}

	return -1
}
