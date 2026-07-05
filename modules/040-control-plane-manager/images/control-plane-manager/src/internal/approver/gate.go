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

package approver

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

const (
	maxPerComponentPerNodeOperations = 1
)

// pipelineStage is one level of the approve chain: components that share the same gate and advance together.
// A pipeline slice (see pipeline.go) is the single source of truth for stage order, membership, and per-stage
// concurrency policy.
type pipelineStage struct {
	components         []controlplanev1alpha1.OperationComponent
	concurrencyLimitFn func(nodes NodeCounts, c controlplanev1alpha1.OperationComponent) int
	// wideBlock marks a stage whose occupancy blocks the whole pipeline regardless of node —
	// e.g. one in-flight etcd operation affects the whole quorum, not just the node it runs on.
	// Defaults to false: block only the same node.
	wideBlock bool
}

// stageGate is the runtime Chain of Responsibility node built from a pipelineStage: it decides whether a
// candidate operation passes or is held back, then hands off to the next gate.
type stageGate struct {
	components map[controlplanev1alpha1.OperationComponent]*componentOccupancy
	wideBlock  bool
	next       *stageGate
}

// componentOccupancy is the reservation bookkeeping for a single component within a gate.
type componentOccupancy struct {
	limit           int
	approvedTotal   int
	approvedPerNode map[string]int
	queuedOnNode    []controlplanev1alpha1.ControlPlaneOperation
}

// buildGateChain builds the linked chain of stageGates from pipeline, computing each stage's
// concurrency limit via stage.concurrencyLimitFn(nodes, component).
func buildGateChain(pipeline []pipelineStage, nodes NodeCounts) *stageGate {
	if len(pipeline) == 0 {
		return nil
	}

	gates := make([]*stageGate, len(pipeline))
	for i, stage := range pipeline {
		components := make(map[controlplanev1alpha1.OperationComponent]*componentOccupancy, len(stage.components))

		for _, c := range stage.components {
			components[c] = &componentOccupancy{
				limit:           stage.concurrencyLimitFn(nodes, c),
				approvedPerNode: make(map[string]int),
			}
		}

		gates[i] = &stageGate{
			components: components,
			wideBlock:  stage.wideBlock,
		}
		if i > 0 {
			gates[i-1].next = gates[i]
		}
	}

	return gates[0]
}

func (gate *stageGate) seedApprovedOperations(approvedOperations []controlplanev1alpha1.ControlPlaneOperation) {
	for _, approvedOperation := range approvedOperations {
		gate.seedApprovedOperation(approvedOperation)
	}
}

func (gate *stageGate) seedApprovedOperation(approvedOperation controlplanev1alpha1.ControlPlaneOperation) {
	if gate == nil {
		return
	}

	if !gate.handles(approvedOperation.Spec.Component) {
		gate.next.seedApprovedOperation(approvedOperation)
		return
	}

	occupancy := gate.components[approvedOperation.Spec.Component]

	occupancy.approvedTotal++
	occupancy.approvedPerNode[approvedOperation.Spec.NodeName]++
}

// tryAdmit attempts to admit unapprovedOperation into the chain, returning true if it can be approved now.
func (gate *stageGate) tryAdmit(unapprovedOperation controlplanev1alpha1.ControlPlaneOperation) bool {
	if gate == nil {
		return false
	}

	if !gate.handles(unapprovedOperation.Spec.Component) {
		if gate.blocks(unapprovedOperation.Spec.NodeName) {
			return false
		}

		return gate.next.tryAdmit(unapprovedOperation)
	}

	occupancy := gate.components[unapprovedOperation.Spec.Component]

	if occupancy.approvedTotal >= occupancy.limit {
		occupancy.queuedOnNode = append(occupancy.queuedOnNode, unapprovedOperation)
		return false
	}

	if occupancy.approvedPerNode[unapprovedOperation.Spec.NodeName] >= maxPerComponentPerNodeOperations {
		return false
	}

	occupancy.approvedTotal++
	occupancy.approvedPerNode[unapprovedOperation.Spec.NodeName]++

	return true
}

func (gate *stageGate) handles(component controlplanev1alpha1.OperationComponent) bool {
	if _, exists := gate.components[component]; !exists {
		return false
	}

	return true
}

// blocks reports whether this gate's current occupancy should hold back the candidate operation:
// any reservation at all for wide-block stages, or a reservation on the same node for ordinary
// per-node stages.
func (gate *stageGate) blocks(nodeName string) bool {
	if gate.wideBlock {
		return gate.hasAnyReservation()
	}

	return gate.hasAnyActiveOperationOnNode(nodeName)
}

func (gate *stageGate) hasAnyReservation() bool {
	for _, occupancy := range gate.components {
		if occupancy.approvedTotal > 0 {
			return true
		}
	}

	return false
}

// hasAnyActiveOperationOnNode returns true if there is any approved or queued operation on the node.
func (gate *stageGate) hasAnyActiveOperationOnNode(nodeName string) bool {
	for _, occupancy := range gate.components {
		if occupancy.approvedPerNode[nodeName] > 0 || occupancy.hasQueuedOnNode(nodeName) {
			return true
		}
	}

	return false
}

func (occ *componentOccupancy) hasQueuedOnNode(nodeName string) bool {
	for _, op := range occ.queuedOnNode {
		if op.Spec.NodeName == nodeName {
			return true
		}
	}

	return false
}
