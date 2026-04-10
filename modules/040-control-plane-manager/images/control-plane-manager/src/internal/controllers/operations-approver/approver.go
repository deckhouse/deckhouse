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

package operationsapprover

import (
	"cmp"
	"slices"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

const (
	maxPerComponentPerNodeOperations = 1
)

var approvePipeline = []pipelineStage{
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentEtcd,
		},
		concurrencyLimitFn: etcdConcurrencyLimit,
	},
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeAPIServer,
		},
		concurrencyLimitFn: controlPlaneWorkloadConcurrencyLimit,
	},
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeControllerManager,
			controlplanev1alpha1.OperationComponentKubeScheduler,
		},
		concurrencyLimitFn: controlPlaneWorkloadConcurrencyLimit,
	},
}

// pipelineStage is one level of the approve chain: components that share the same link and advance together.
// This slice is the single source of truth for stage order, membership, and per-stage concurrency policy.
type pipelineStage struct {
	components         []controlplanev1alpha1.OperationComponent
	concurrencyLimitFn func(nodesCount int) int
}

// pipelineStageIndex returns the stage index of c in controlPlaneApprovePipeline, or -1 if unknown.
func pipelineStageIndex(c controlplanev1alpha1.OperationComponent) int {
	for i, stage := range approvePipeline {
		if slices.Contains(stage.components, c) {
			return i
		}
	}

	return -1
}

type approver struct {
	approveChain *approveLink
	approveQueue []controlplanev1alpha1.ControlPlaneOperation
}

type approveLink struct {
	components map[controlplanev1alpha1.OperationComponent]*component
	nextLink   *approveLink
}

type component struct {
	concurrencyLimit          int
	approvedOperationsTotal   int
	approvedOperationsPerNode map[string]int
}

// newApprover builds an approver for one reconcile pass: partitions operations into
// approved && !Completed and unapproved, sorts both by pipeline stage,
// seeds the chain, and exposes unapproved operations via approveQueue iteration order.
func newApprover(nodesCount int, operations []controlplanev1alpha1.ControlPlaneOperation) *approver {
	approvedOperations, unapprovedOperations := partitionOperationsByApprovalState(operations)
	sortOperationsByPipelineOrder(approvedOperations)
	sortOperationsByPipelineOrder(unapprovedOperations)

	approveChain := buildApproveChain(nodesCount)
	approveChain.seedApprovedOperations(approvedOperations)

	return &approver{
		approveChain: approveChain,
		approveQueue: unapprovedOperations,
	}
}

func partitionOperationsByApprovalState(operations []controlplanev1alpha1.ControlPlaneOperation) (approvedOperations, unapprovedOperations []controlplanev1alpha1.ControlPlaneOperation) {
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

// sortOperationsByPipelineOrder orders operations by pipelineStageIndex (see controlPlaneApprovePipeline).
// Within the same stage, order is stable by resource name.
func sortOperationsByPipelineOrder(operations []controlplanev1alpha1.ControlPlaneOperation) {
	slices.SortFunc(operations, func(a, b controlplanev1alpha1.ControlPlaneOperation) int {
		if c := cmp.Compare(pipelineStageIndex(a.Spec.Component), pipelineStageIndex(b.Spec.Component)); c != 0 {
			return c
		}
		return cmp.Compare(a.Name, b.Name)
	})
}

func buildApproveChain(nodesCount int) *approveLink {
	if len(approvePipeline) == 0 {
		return nil
	}

	links := make([]*approveLink, len(approvePipeline))
	for i, stage := range approvePipeline {
		limit := stage.concurrencyLimitFn(nodesCount)
		components := make(map[controlplanev1alpha1.OperationComponent]*component, len(stage.components))

		for _, c := range stage.components {
			components[c] = &component{
				concurrencyLimit:          limit,
				approvedOperationsPerNode: make(map[string]int, nodesCount),
			}
		}

		links[i] = &approveLink{components: components}
		if i > 0 {
			links[i-1].nextLink = links[i]
		}
	}

	return links[0]
}

func (a *approver) tryApprove(operation controlplanev1alpha1.ControlPlaneOperation) bool {
	return a.approveChain.tryReserveApproval(operation)
}

func (link *approveLink) seedApprovedOperations(approvedOperations []controlplanev1alpha1.ControlPlaneOperation) {
	for _, approvedOperation := range approvedOperations {
		link.seedApprovedOperation(approvedOperation)
	}
}

func (link *approveLink) seedApprovedOperation(approvedOperation controlplanev1alpha1.ControlPlaneOperation) {
	if link == nil {
		return
	}

	if !link.containsComponent(approvedOperation.Spec.Component) {
		link.nextLink.seedApprovedOperation(approvedOperation)
		return
	}

	component := link.components[approvedOperation.Spec.Component]

	if _, exists := component.approvedOperationsPerNode[approvedOperation.Spec.NodeName]; !exists {
		component.approvedOperationsPerNode[approvedOperation.Spec.NodeName] = 0
	}

	component.approvedOperationsTotal++
	component.approvedOperationsPerNode[approvedOperation.Spec.NodeName] = component.approvedOperationsPerNode[approvedOperation.Spec.NodeName] + 1
}

func (link *approveLink) tryReserveApproval(unapprovedOperation controlplanev1alpha1.ControlPlaneOperation) bool {
	if link == nil {
		return false
	}

	if !link.containsComponent(unapprovedOperation.Spec.Component) {
		if link.hasAnyApprovedOperation() {
			return false
		}

		return link.nextLink.tryReserveApproval(unapprovedOperation)
	}

	component := link.components[unapprovedOperation.Spec.Component]

	if component.approvedOperationsTotal >= component.concurrencyLimit {
		return false
	}

	if _, exists := component.approvedOperationsPerNode[unapprovedOperation.Spec.NodeName]; !exists {
		component.approvedOperationsPerNode[unapprovedOperation.Spec.NodeName] = 0
	}

	if component.approvedOperationsPerNode[unapprovedOperation.Spec.NodeName] >= maxPerComponentPerNodeOperations {
		return false
	}

	component.approvedOperationsTotal++
	component.approvedOperationsPerNode[unapprovedOperation.Spec.NodeName] = component.approvedOperationsPerNode[unapprovedOperation.Spec.NodeName] + 1

	return true
}

func (link *approveLink) containsComponent(component controlplanev1alpha1.OperationComponent) bool {
	if _, exists := link.components[component]; !exists {
		return false
	}

	return true
}

func (link *approveLink) hasAnyApprovedOperation() bool {
	for _, component := range link.components {
		if component.approvedOperationsTotal > 0 {
			return true
		}
	}

	return false
}

func etcdConcurrencyLimit(nodesCount int) int {
	_ = nodesCount
	return 1
}

func controlPlaneWorkloadConcurrencyLimit(nodesCount int) int {
	return max(1, nodesCount-1)
}
