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
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"time"
)

type StepExecutor interface {
	Execute(ctx context.Context, stepName controlplanev1alpha1.StepName) StepResult
}

type StepStatus int

const (
	StepProgressing StepStatus = iota
	StepCompleted
	StepFailed
)

type StepResult struct {
	Name           controlplanev1alpha1.StepName
	Status         StepStatus
	Message        string
	RequeueAfter   time.Duration
	Error          error
	OperationFuncs []func(operation *controlplanev1alpha1.ControlPlaneOperation)
}

func StepIsCompleted(stepName controlplanev1alpha1.StepName, message string, operationFuncs ...func(operation *controlplanev1alpha1.ControlPlaneOperation)) StepResult {
	return StepResult{
		Name:           stepName,
		Status:         StepCompleted,
		Message:        message,
		OperationFuncs: operationFuncs,
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
