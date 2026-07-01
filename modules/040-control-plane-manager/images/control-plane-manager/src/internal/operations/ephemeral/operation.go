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
	"control-plane-manager/internal/constants"
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

func (e *OperationExecutor) Execute(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) operations.OperationResult {
	steps := filterSteps(operation)

	stepExecutor := StepExecutor{
		client:            e.client,
		operation:         operation,
		tenantIdentity:    tenantIdentityFromOperation(operation),
		clusterDomain:     constants.DefaultTenantClusterDomain,
		serviceSubnetCIDR: constants.DefaultTenantServiceSubnetCIDR,
	}

	var stepResults []operations.StepResult
	for _, stepName := range steps {
		stepResult := stepExecutor.Execute(ctx, stepName)
		stepResults = append(stepResults, stepResult)

		if stepResult.Status == operations.StepFailed || stepResult.Status == operations.StepProgressing {
			break
		}
	}

	return operations.NewOperationResult(stepResults)
}

func filterSteps(operation *controlplanev1alpha1.ControlPlaneOperation) []controlplanev1alpha1.StepName {
	steps := make([]controlplanev1alpha1.StepName, 0)
	for _, step := range operation.Spec.Steps {
		if operation.IsStepCompleted(step) {
			continue
		}
		steps = append(steps, step)
	}

	return steps
}
