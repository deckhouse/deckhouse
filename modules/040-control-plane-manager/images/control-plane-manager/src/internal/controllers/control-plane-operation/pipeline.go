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

package controlplaneoperation

import (
	"context"
	"fmt"
	"log/slog"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"

	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ClusterSecrets struct {
	CPMData map[string][]byte
	PKIData map[string][]byte
}

// StepEnv is the input data for step execution: operation state, secrets, and node identity.
// Steps may mutate State; the pipeline handles all status flushing.
type StepEnv struct {
	State   *controlplanev1alpha1.OperationState
	Secrets ClusterSecrets
	Node    NodeIdentity
}

// reconcilePipeline executes the step-based pipeline for component operations.
// Completed steps (condition=True) are skipped on requeue.
// On step failure the operation is marked failed and becomes terminal.
func (r *Reconciler) reconcilePipeline(ctx context.Context, state *controlplanev1alpha1.OperationState, secrets ClusterSecrets, logger *log.Logger) (reconcile.Result, error) {
	stepNames := state.Raw().Spec.Steps
	if err := resolveSteps(r.steps, stepNames); err != nil {
		return reconcile.Result{}, err
	}

	env := &StepEnv{
		State:   state,
		Secrets: secrets,
		Node:    r.node,
	}

	for _, name := range stepNames {
		if state.IsStepCompleted(name) {
			logger.With(slog.String("step", string(name))).Info("step already completed, skipping")
			continue
		}

		state.MarkOperationInProgress(fmt.Sprintf("executing step %s", name))
		syncOperationExecutionMetrics(state.Raw())
		step := r.steps[name]
		result, err := r.executeStep(ctx, state, name, step, env, logger)
		if err != nil {
			state.MarkOperationFailed(err.Error())
			if patchErr := r.patchStatus(ctx, state); patchErr != nil {
				logger.Error("failed to persist operation failure", log.Err(patchErr))
			}
			return result, err
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	state.MarkOperationCompleted()
	return reconcile.Result{}, r.patchStatus(ctx, state)
}

// executeStep runs a single pipeline step with status tracking and start/finish logging.
func (r *Reconciler) executeStep(ctx context.Context, state *controlplanev1alpha1.OperationState, name controlplanev1alpha1.StepName, step Step, env *StepEnv, logger *log.Logger) (result reconcile.Result, err error) {
	stepLogger := logger.With(slog.String("step", string(name)))
	var execErr error

	stepLogger.Info("executing step")
	defer func() {
		if recovered := recover(); recovered != nil {
			execErr = fmt.Errorf("panic in step %s: %v", name, recovered)
		}

		switch {
		case execErr != nil:
			state.MarkStepFailed(name, execErr.Error())
			err = execErr
		case result.RequeueAfter > 0:
			if patchErr := r.patchStatus(ctx, state); patchErr != nil {
				stepLogger.Warn("failed to flush status on requeue", log.Err(patchErr))
			}
			err = nil
		default:
			// Step may already have a completed condition (with extra details in Message), don't override it.
			cond := state.Raw().GetCondition(controlplanev1alpha1.StepConditionType(name))
			if cond == nil || cond.Reason != controlplanev1alpha1.CPOReasonStepCompleted {
				state.MarkStepCompleted(name)
			}
			if patchErr := r.patchStatus(ctx, state); patchErr != nil {
				err = fmt.Errorf("set completed condition for %s: %w", name, patchErr)
			} else {
				err = nil
			}
		}

		if err != nil {
			stepLogger.Error("step failed", log.Err(err))
		} else {
			stepLogger.Info("step finished")
		}
	}()

	state.MarkStepInProgress(name)
	if patchErr := r.patchStatus(ctx, state); patchErr != nil {
		if isCommitPointStep(name) {
			return reconcile.Result{}, fmt.Errorf("set in-progress condition for commit-point step %s: %w", name, patchErr)
		}
		stepLogger.Warn("failed to set in-progress condition", log.Err(patchErr))
	}

	result, execErr = step.Execute(ctx, env, stepLogger)
	return result, execErr
}

func isCommitPointStep(name controlplanev1alpha1.StepName) bool {
	switch name {
	case controlplanev1alpha1.StepSyncManifests,
		controlplanev1alpha1.StepJoinEtcdCluster:
		return true
	default:
		return false
	}
}

// patchStatus flushes OperationState status changes to the API server.
func (r *Reconciler) patchStatus(ctx context.Context, state *controlplanev1alpha1.OperationState) error {
	if !state.HasStatusChanges() {
		return nil
	}
	if err := r.client.Status().Patch(ctx, state.Raw(), client.MergeFrom(state.Original())); err != nil {
		return err
	}
	state.ResetBaseline()
	return nil
}
