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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ClusterSecrets struct {
	CPMData map[string][]byte
	PKIData map[string][]byte
}

// CommandEnv is the input data for command execution: operation state, secrets, and node identity.
// Commands may mutate State; the pipeline handles all status flushing.
type CommandEnv struct {
	State   *controlplanev1alpha1.OperationState
	Secrets ClusterSecrets
	Node    NodeIdentity
}

// reconcilePipeline executes the command-based pipeline for component operations.
// Completed commands (condition=True) are skipped on requeue.
// On command failure the operation is marked failed and becomes terminal.
func (r *Reconciler) reconcilePipeline(ctx context.Context, state *controlplanev1alpha1.OperationState, secrets ClusterSecrets, logger *log.Logger) (reconcile.Result, error) {
	commandNames := state.Raw().Spec.Commands
	if err := resolveCommands(r.commands, commandNames); err != nil {
		return reconcile.Result{}, err
	}

	env := &CommandEnv{
		State:   state,
		Secrets: secrets,
		Node:    r.node,
	}

	for _, name := range commandNames {
		if state.IsCommandCompleted(name) {
			logger.With(slog.String("command", string(name))).Info("command already completed, skipping")
			continue
		}

		state.MarkOperationInProgress(fmt.Sprintf("executing command %s", name))
		syncOperationExecutionMetrics(state.Raw())
		cmd := r.commands[name]
		result, err := r.executeCommand(ctx, state, name, cmd, env, logger)
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

// executeCommand runs a single pipeline command with status tracking and start/finish logging.
func (r *Reconciler) executeCommand(ctx context.Context, state *controlplanev1alpha1.OperationState, name controlplanev1alpha1.CommandName, cmd Command, env *CommandEnv, logger *log.Logger) (result reconcile.Result, err error) {
	cmdLogger := logger.With(slog.String("command", string(name)))
	var execErr error

	cmdLogger.Info("executing command")
	defer func() {
		if recovered := recover(); recovered != nil {
			execErr = fmt.Errorf("panic in command %s: %v", name, recovered)
		}

		switch {
		case execErr != nil:
			state.MarkCommandFailed(name, execErr.Error())
			err = execErr
		case result.RequeueAfter > 0:
			if patchErr := r.patchStatus(ctx, state); patchErr != nil {
				cmdLogger.Warn("failed to flush status on requeue", log.Err(patchErr))
			}
			err = nil
		default:
			// Command may already have a completed condition (with extra details in Message), don't override it.
			cond := state.Raw().GetCondition(string(name))
			if cond == nil || cond.Status != metav1.ConditionTrue {
				state.MarkCommandCompleted(name)
			}
			if patchErr := r.patchStatus(ctx, state); patchErr != nil {
				err = fmt.Errorf("set completed condition for %s: %w", name, patchErr)
			} else {
				err = nil
			}
		}

		if err != nil {
			cmdLogger.Error("command failed", log.Err(err))
		} else {
			cmdLogger.Info("command finished")
		}
	}()

	state.MarkCommandInProgress(name)
	if patchErr := r.patchStatus(ctx, state); patchErr != nil {
		if isCommitPointCommand(name) {
			return reconcile.Result{}, fmt.Errorf("set in-progress condition for commit-point command %s: %w", name, patchErr)
		}
		cmdLogger.Warn("failed to set in-progress condition", log.Err(patchErr))
	}

	result, execErr = cmd.Execute(ctx, env, cmdLogger)
	return result, execErr
}

func isCommitPointCommand(name controlplanev1alpha1.CommandName) bool {
	switch name {
	case controlplanev1alpha1.CommandSyncManifests,
		controlplanev1alpha1.CommandJoinEtcdCluster:
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
