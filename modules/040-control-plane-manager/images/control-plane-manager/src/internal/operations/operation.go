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

// Package operations is the ControlPlaneOperation domain: building an operation from explicit inputs,
// telling whether an active operation already covers a needed step, and rotating terminal operations.
package operations

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
)

type NodeRef struct {
	Namespace string
	Name      string
	Type      string // control-plane.deckhouse.io/type label value
	UID       types.UID
}

func New(node NodeRef, c controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName, intended controlplanev1alpha1.Checksums, approved bool) *controlplanev1alpha1.ControlPlaneOperation {
	op := newOperation(node, c, steps)
	if approved {
		op.Spec.Approved = true
	} else {
		op.Spec.DesiredConfigChecksum = intended.Config
		op.Spec.DesiredPKIChecksum = intended.PKI
		op.Spec.DesiredCAChecksum = intended.CA
	}
	op.GenerateName = generateName(op)
	return op
}

func newOperation(node NodeRef, component controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName) *controlplanev1alpha1.ControlPlaneOperation {
	return &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: node.Namespace,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  node.Name,
				constants.ControlPlaneComponentLabelKey: component.LabelValue(),
				constants.ControlPlaneTypeLabelKey:      node.Type,
				constants.HeritageLabelKey:              constants.HeritageLabelValue,
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:  node.Name,
			Component: component,
			Steps:     steps,
		},
	}
}

func generateName(op *controlplanev1alpha1.ControlPlaneOperation) string {
	name := strings.ToLower(string(op.Spec.Component))
	for _, ck := range []string{
		op.Spec.DesiredConfigChecksum,
		op.Spec.DesiredPKIChecksum,
		op.Spec.DesiredCAChecksum,
	} {
		if ck != "" {
			name += "-" + checksum.ShortChecksum(ck)
		}
	}
	return name + "-"
}

func StepCoveredByActiveOperation(current []controlplanev1alpha1.ControlPlaneOperation, c controlplanev1alpha1.OperationComponent, step controlplanev1alpha1.StepName) bool {
	for i := range current {
		op := &current[i]
		if op.IsTerminal() || op.Spec.Component != c {
			continue
		}
		if op.HasStep(step) {
			return true
		}
	}
	return false
}
