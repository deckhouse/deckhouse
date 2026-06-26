package ephemeral

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EphemeralExecutor struct {
	client client.Client
}

func NewEphemeralExecutor(client client.Client) *EphemeralExecutor {
	return &EphemeralExecutor{
		client: client,
	}
}

func (e *EphemeralExecutor) NeedsExecution(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (bool, string) {
	if obsolete, reason := e.isObsolete(ctx, operation); obsolete {
		return false, reason
	}

	return true, ""
}

func (e *EphemeralExecutor) Execute(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) operations.OperationResult {
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

func (e *EphemeralExecutor) isObsolete(ctx context.Context, operation *controlplanev1alpha1.ControlPlaneOperation) (bool, string) {
	return false, ""
}
