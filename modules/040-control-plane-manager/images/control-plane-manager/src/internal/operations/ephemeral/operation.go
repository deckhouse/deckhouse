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

package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OperationExecutor struct {
	client client.Client
}

func NewOperationExecutor(client client.Client) *OperationExecutor {
	return &OperationExecutor{
		client: client,
	}
}

func (e *OperationExecutor) NeedsExecution(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (bool, string) {
	if obsolete, reason := e.isObsolete(ctx, operation); obsolete {
		return false, reason
	}

	return true, ""
}

func (e *OperationExecutor) Execute(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) operations.OperationResult {
	stepExecutor := StepExecutor{
		client:         e.client,
		operation:      operation,
		tenantIdentity: tenantIdentityFromOperation(operation),
	}

	var steps []operations.StepResult
	for _, stepName := range operation.Spec.Steps {
		stepResult := stepExecutor.Execute(ctx, stepName)
		steps = append(steps, stepResult)

		if stepResult.Status == operations.StepFailed || stepResult.Status == operations.StepProgressing {
			break
		}
	}

	return operations.NewOperationResult(steps)
}

func (e *OperationExecutor) isObsolete(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (bool, string) {
	return false, ""
}
