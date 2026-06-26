package operations

import (
	"context"
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"time"
)

type StepExecutor interface {
	Execute(ctx context.Context, stepName controlplanev1alpha1.StepName) StepResult
}

// StepStatus is the outcome of one execution attempt. Exactly one value holds
// per StepResult, so illegal combos (e.g. completed+error) are unrepresentable.
type StepStatus int

const (
	// StepProgressing: desired not yet observed; converge again later (RequeueAfter).
	StepProgressing StepStatus = iota
	// StepCompleted: desired state observed in place; advance to the next step.
	StepCompleted
	// StepFailed: the step errored; the operation fails (and retries with backoff).
	StepFailed
)

type StepResult struct {
	Name         controlplanev1alpha1.StepName
	Status       StepStatus
	Message      string
	RequeueAfter time.Duration
	Error        error
}

func StepIsCompleted(stepName controlplanev1alpha1.StepName, message string) StepResult {
	return StepResult{
		Name:    stepName,
		Status:  StepCompleted,
		Message: message,
	}
}
func StepIsProgressing(stepName controlplanev1alpha1.StepName, message string, requeueAfter time.Duration) StepResult {
	return StepResult{
		Name:         stepName,
		Status:       StepProgressing,
		Message:      message,
		RequeueAfter: requeueAfter,
	}
}
func StepHasFailed(stepName controlplanev1alpha1.StepName, err error) StepResult {
	return StepResult{
		Name:    stepName,
		Status:  StepFailed,
		Error:   err,
		Message: err.Error(),
	}
}
