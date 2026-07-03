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
	"context"
	"strings"
	"time"

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

func NewOperation(node NodeRef, c controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName, intended controlplanev1alpha1.Checksums) *controlplanev1alpha1.ControlPlaneOperation {
	op := baseOperation(node, c, steps)
	op.Spec.DesiredConfigChecksum = intended.Config
	op.Spec.DesiredPKIChecksum = intended.PKI
	op.Spec.DesiredCAChecksum = intended.CA
	op.Spec.Approved = true // TODO(virtual): remove after implementing approval mechanism
	op.GenerateName = generateName(op)
	return op
}

func NewApprovedOperation(node NodeRef, c controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName) *controlplanev1alpha1.ControlPlaneOperation {
	op := baseOperation(node, c, steps)
	op.Spec.Approved = true
	op.GenerateName = generateName(op)
	return op
}

func baseOperation(node NodeRef, component controlplanev1alpha1.OperationComponent, steps []controlplanev1alpha1.StepName) *controlplanev1alpha1.ControlPlaneOperation {
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

type OperationExecutor interface {
	Execute(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) OperationResult
}

type OperationOutcome int

const (
	OperationInProgress OperationOutcome = iota
	OperationCompleted
	OperationFailed
)

type OperationResult struct {
	Outcome        OperationOutcome
	Message        string
	RequeueAfter   time.Duration
	Error          error
	StepResults    []StepResult
	OperationFuncs []func(operation *controlplanev1alpha1.ControlPlaneOperation)
}

func NewOperationResult(steps []StepResult) OperationResult {
	result := OperationResult{
		Outcome:     OperationCompleted,
		StepResults: steps,
	}
	if len(steps) == 0 {
		return result
	}

	for _, step := range steps {
		for _, fn := range step.OperationFuncs {
			result.OperationFuncs = append(result.OperationFuncs, fn)
		}
	}

	switch last := steps[len(steps)-1]; last.Status {
	case StepFailed:
		result.Outcome = OperationFailed
		result.Message = last.Message
		result.Error = last.Error
	case StepProgressing:
		result.Outcome = OperationInProgress
		result.Message = last.Message
		result.RequeueAfter = last.RequeueAfter
	default:
		result.Message = last.Message
	}

	return result
}
