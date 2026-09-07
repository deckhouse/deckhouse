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

package operations

import (
	"sort"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

// ComputeOperationsToRotate returns terminal operations exceeding the per-component retention limit, oldest first.
// Active operations are never rotated.
func ComputeOperationsToRotate(current []controlplanev1alpha1.ControlPlaneOperation, keepPerComponent int) []*controlplanev1alpha1.ControlPlaneOperation {
	if keepPerComponent <= 0 {
		return nil
	}

	terminalByComponent := make(map[controlplanev1alpha1.OperationComponent][]*controlplanev1alpha1.ControlPlaneOperation)
	for i := range current {
		op := &current[i]
		if op.IsTerminal() {
			terminalByComponent[op.Spec.Component] = append(terminalByComponent[op.Spec.Component], op)
		}
	}

	var toDelete []*controlplanev1alpha1.ControlPlaneOperation
	for _, ops := range terminalByComponent {
		if len(ops) <= keepPerComponent {
			continue
		}
		sort.SliceStable(ops, func(i, j int) bool {
			ti, tj := ops[i].CreationTimestamp.Time, ops[j].CreationTimestamp.Time
			if ti.Equal(tj) {
				return ops[i].Name < ops[j].Name
			}
			return ti.Before(tj)
		})
		toDelete = append(toDelete, ops[:len(ops)-keepPerComponent]...)
	}

	sort.SliceStable(toDelete, func(i, j int) bool { return toDelete[i].Name < toDelete[j].Name })
	return toDelete
}

func FilterOwnedBy(ops []controlplanev1alpha1.ControlPlaneOperation, node NodeRef) []controlplanev1alpha1.ControlPlaneOperation {
	filtered := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	for i := range ops {
		if isOwnedBy(&ops[i], node) {
			filtered = append(filtered, ops[i])
		}
	}
	return filtered
}

func isOwnedBy(op *controlplanev1alpha1.ControlPlaneOperation, node NodeRef) bool {
	for i := range op.OwnerReferences {
		ref := op.OwnerReferences[i]
		if ref.APIVersion != controlplanev1alpha1.GroupVersion.String() || ref.Kind != "ControlPlaneNode" {
			continue
		}
		if ref.Name != node.Name || ref.UID != node.UID {
			continue
		}
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}
