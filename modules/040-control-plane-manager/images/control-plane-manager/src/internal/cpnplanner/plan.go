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
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
)

const maxTerminalOperationsPerComponent = 5

// RequiredOperations returns the operations the node needs that are not already covered by an active operation.
func RequiredOperations(cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation) []*controlplanev1alpha1.ControlPlaneOperation {
	node := nodeRef(cpn)
	var out []*controlplanev1alpha1.ControlPlaneOperation
	for _, d := range decisions(cpn) {
		if operations.Covered(current, d) {
			continue
		}
		out = append(out, operations.Build(node, d))
	}
	return out
}

func OwnedOperations(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) []controlplanev1alpha1.ControlPlaneOperation {
	return operations.FilterOwnedBy(ops, nodeRef(cpn))
}

func OperationsToRotate(ops []controlplanev1alpha1.ControlPlaneOperation) []string {
	return operations.ComputeOperationsToRotate(ops, maxTerminalOperationsPerComponent)
}

func IsMaintenanceMode(cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	_, ok := cpn.Labels[constants.MaintenanceModeLabelKey]
	return ok
}

// decisions translates the node's component states into operation decisions.
//
// Two independent choices per component:
//   - lifecycle: a mutating converge (spec drift) or a read-only observe — mutually exclusive;
//   - renewal: an expiring leaf certificate or signature key — runs in parallel to the lifecycle.
func decisions(cpn *controlplanev1alpha1.ControlPlaneNode) []operations.Decision {
	var ds []operations.Decision
	for _, s := range computeComponentStates(cpn) {
		switch {
		case s.needsConverge():
			ds = append(ds, operations.ConvergeDecision(s.component, s.intended, s.certsChanged(), s.needsSignatureBootstrap()))
		case s.needsObserve():
			ds = append(ds, operations.ObserveDecision(s.component))
		}

		switch {
		case s.needsCertRenew():
			ds = append(ds, operations.CertRenewDecision(s.component, s.intended))
		case s.needsSignatureRenew():
			ds = append(ds, operations.SignatureRenewDecision(s.component, s.intended))
		}
	}
	return ds
}

func nodeRef(cpn *controlplanev1alpha1.ControlPlaneNode) operations.NodeRef {
	return operations.NodeRef{
		Namespace: cpn.Namespace,
		Name:      cpn.Name,
		Type:      cpn.Labels[constants.ControlPlaneTypeLabelKey],
		UID:       cpn.UID,
	}
}
