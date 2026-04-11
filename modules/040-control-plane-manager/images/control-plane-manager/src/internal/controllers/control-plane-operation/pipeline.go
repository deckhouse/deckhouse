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

// CommandEnv is the input data for command execution: operation state, secrets, and node identity.
// Commands may mutate State; the pipeline handles all status flushing.
type CommandEnv struct {
	State         *controlplanev1alpha1.OperationState
	CPMSecretData map[string][]byte
	PKISecretData map[string][]byte
	Node          NodeIdentity
}

// reconcilePipeline executes the command-based pipeline for component operations.
// Completed commands (condition=True) are skipped on requeue.
// On command failure the failed command condition stays False, so it re-executes on next reconcile.
func (r *Reconciler) reconcilePipeline(ctx context.Context, state *controlplanev1alpha1.OperationState, cpmSecretData, pkiSecretData map[string][]byte, logger *log.Logger) (reconcile.Result, error) {
	commandNames := state.Raw().Spec.Commands
	if err := resolveCommands(r.commands, commandNames); err != nil {
		return reconcile.Result{}, err
	}

	env := &CommandEnv{
		State:         state,
		CPMSecretData: cpmSecretData,
		PKISecretData: pkiSecretData,
		Node:          r.node,
	}

	for _, name := range commandNames {
		cmd := r.commands[name]
		result, err := r.executeCommand(ctx, state, name, cmd, env, logger)
		if err != nil {
			return result, err
		}
		if result.RequeueAfter > 0 {
			return result, nil
		}
	}

	// All commands completed successfully — mark operation as ready.
	// For static pod components this is typically done by WaitPodReady,
	// but for CA/HotReload the pipeline may not include WaitPodReady.
	if !state.IsCompleted() {
		state.MarkSucceeded()
		return reconcile.Result{}, r.patchStatus(ctx, state)
	}

	return reconcile.Result{}, nil
}

// executeCommand runs a single pipeline command with status tracking and start/finish logging.
func (r *Reconciler) executeCommand(ctx context.Context, state *controlplanev1alpha1.OperationState, name controlplanev1alpha1.CommandName, cmd Command, env *CommandEnv, logger *log.Logger) (result reconcile.Result, err error) {
	cmdLogger := logger.With(slog.String("command", string(name)))

	if state.IsCommandCompleted(name) {
		cmdLogger.Info("command already completed, skipping")
		return reconcile.Result{}, nil
	}

	cmdLogger.Info("executing command")
	defer func() {
		if err != nil {
			cmdLogger.Error("command failed", log.Err(err))
		} else {
			cmdLogger.Info("command finished")
		}
	}()

	state.MarkCommandInProgress(name)
	state.SetReadyReason(commandReadyReasons[name], fmt.Sprintf("executing command %s", name))
	if patchErr := r.patchStatus(ctx, state); patchErr != nil {
		if isCommitPointCommand(name) {
			return reconcile.Result{}, fmt.Errorf("set in-progress condition for commit-point command %s: %w", name, patchErr)
		}
		cmdLogger.Warn("failed to set in-progress condition", log.Err(patchErr))
	}

	result, err = cmd.Execute(ctx, env, cmdLogger)
	if err != nil {
		state.MarkCommandFailed(name, err.Error())
		if setErr := r.patchStatus(ctx, state); setErr != nil {
			cmdLogger.Error("failed to set failed condition", log.Err(setErr))
		}
		return result, err
	}

	if result.RequeueAfter > 0 {
		if patchErr := r.patchStatus(ctx, state); patchErr != nil {
			cmdLogger.Warn("failed to flush status on requeue", log.Err(patchErr))
		}
		return result, nil
	}

	state.MarkCommandCompleted(name)
	if err = r.patchStatus(ctx, state); err != nil {
		return result, fmt.Errorf("set completed condition for %s: %w", name, err)
	}
	return result, nil
}

func isCommitPointCommand(name controlplanev1alpha1.CommandName) bool {
	switch name {
	case controlplanev1alpha1.CommandSyncManifests,
		controlplanev1alpha1.CommandJoinEtcdCluster,
		controlplanev1alpha1.CommandSyncHotReload:
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
