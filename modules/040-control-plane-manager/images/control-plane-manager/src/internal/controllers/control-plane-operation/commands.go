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

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// podWaiter waits for static pod readiness.
type podWaiter interface {
	waitForPod(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error)
}

// Compile-time checks.
var (
	_ podWaiter = (*Reconciler)(nil)

	_ Command = (*syncCACommand)(nil)
	_ Command = (*renewPKICertsCommand)(nil)
	_ Command = (*renewKubeconfigsCommand)(nil)
	_ Command = (*syncManifestsCommand)(nil)
	_ Command = (*joinEtcdClusterCommand)(nil)
	_ Command = (*waitPodReadyCommand)(nil)
	_ Command = (*syncHotReloadCommand)(nil)
	_ Command = (*certObserveCommand)(nil)
)

// CommandEnv is everything a command needs to run: operation state, node identity,
// and a flush callback for commands that need to persist status mid-execution.
type CommandEnv struct {
	State         *controlplanev1alpha1.OperationState
	CPMSecretData map[string][]byte
	PKISecretData map[string][]byte
	Node          NodeIdentity
	FlushStatus   func(ctx context.Context) error
}

// Command is the interface each pipeline step must satisfy.
type Command interface {
	CommandName() controlplanev1alpha1.CommandName
	ReadyReason() string
	Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error)
}

// baseCommand provides shared Name/ReadyReason for all commands.
type baseCommand struct {
	name        controlplanev1alpha1.CommandName
	readyReason string
}

func (b baseCommand) CommandName() controlplanev1alpha1.CommandName { return b.name }
func (b baseCommand) ReadyReason() string                           { return b.readyReason }

// defaultCommands returns a fresh command registry with all known commands.
// Reconciler-level deps (podWaiter) must be injected after construction.
func defaultCommands() map[controlplanev1alpha1.CommandName]Command {
	cmds := []Command{
		&syncCACommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandSyncCA, readyReason: constants.ReasonSyncingCA}},
		&renewPKICertsCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandRenewPKICerts, readyReason: constants.ReasonRenewingPKI}},
		&renewKubeconfigsCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandRenewKubeconfigs, readyReason: constants.ReasonRenewingKubeconfigs}},
		&syncManifestsCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandSyncManifests, readyReason: constants.ReasonSyncingManifests}},
		&joinEtcdClusterCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandJoinEtcdCluster, readyReason: constants.ReasonJoiningEtcd}},
		&waitPodReadyCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandWaitPodReady, readyReason: constants.ReasonWaitingForPod}},
		&syncHotReloadCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandSyncHotReload, readyReason: constants.ReasonSyncingHotReload}},
		&certObserveCommand{baseCommand: baseCommand{name: controlplanev1alpha1.CommandCertObserve, readyReason: constants.ReasonCertObserving}},
	}
	registry := make(map[controlplanev1alpha1.CommandName]Command, len(cmds))
	for _, cmd := range cmds {
		registry[cmd.CommandName()] = cmd
	}
	return registry
}

// resolveCommands looks up Command entries for the given command names.
func resolveCommands(registry map[controlplanev1alpha1.CommandName]Command, names []controlplanev1alpha1.CommandName) ([]Command, error) {
	commands := make([]Command, 0, len(names))
	for _, name := range names {
		cmd, ok := registry[name]
		if !ok {
			return nil, fmt.Errorf("unknown command: %s", name)
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

// syncCACommand installs CA files from the d8-pki secret to disk.
type syncCACommand struct{ baseCommand }

func (c *syncCACommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("installing CA files from secret")
	if err := installCAsFromSecret(env.PKISecretData, constants.KubernetesPkiPath); err != nil {
		logger.Error("failed to install CAs", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// renewPKICertsCommand renews leaf certificates for the component.
// No-op for KCM/Scheduler (certTree=nil).
type renewPKICertsCommand struct{ baseCommand }

func (c *renewPKICertsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	certTree := certTreeForComponent(env.State.Raw().Spec.Component)
	if certTree != nil {
		logger.Info("renewing leaf certificates if needed")
		params := parsePKIParams(constants.KubernetesPkiPath, env.CPMSecretData, env.Node)
		if err := renewCertsIfNeeded(params, certTree); err != nil {
			logger.Error("failed to renew certs", log.Err(err))
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// renewKubeconfigsCommand renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
type renewKubeconfigsCommand struct{ baseCommand }

func (c *renewKubeconfigsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	component := env.State.Raw().Spec.Component
	kubeconfigDir := env.Node.KubeconfigDir
	if err := renewKubeconfigsForComponent(component, env.CPMSecretData, constants.KubernetesPkiPath, kubeconfigDir, env.Node.AdvertiseIP); err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, err
	}
	// dont return error if failed to update root kubeconfig symlink, maybe return reconcile.Result{}, err later.
	if needsRootKubeconfig(component) {
		if err := updateRootKubeconfig(kubeconfigDir, env.Node.HomeDir); err != nil {
			logger.Warn("failed to update root kubeconfig symlink", log.Err(err))
		}
	}
	return reconcile.Result{}, nil
}

// joinEtcdClusterCommand checks if etcd needs to join the cluster and handles the full join flow.
// No-op for non-etcd components. No-op if already in cluster.
type joinEtcdClusterCommand struct{ baseCommand }

func (c *joinEtcdClusterCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	if op.Spec.Component != controlplanev1alpha1.OperationComponentEtcd {
		return reconcile.Result{}, nil
	}

	kubeconfigDir := env.Node.KubeconfigDir

	// Ensure admin.conf exists before checking membership on fresh nodes (including etcd-arbiter)
	if err := ensureAdminKubeconfig(env.CPMSecretData, constants.KubernetesPkiPath, kubeconfigDir, env.Node.AdvertiseIP); err != nil {
		logger.Error("failed to ensure admin kubeconfig", log.Err(err))
		return reconcile.Result{}, fmt.Errorf("ensure admin kubeconfig: %w", err)
	}

	needsJoin, err := etcdNeedsJoin(env.Node, constants.KubernetesPkiPath, kubeconfigDir)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("check etcd join need: %w", err)
	}
	if !needsJoin {
		return reconcile.Result{}, nil
	}
	logger.Info("etcd needs join, executing join flow")
	return reconcileEtcdJoin(env.Node, op.Spec.Component, env.CPMSecretData,
		op.Spec.DesiredConfigChecksum, op.Spec.DesiredPKIChecksum, op.Spec.DesiredCAChecksum, logger)
}

// syncManifestsCommand writes the static pod manifest (or patches annotations for PKI-only updates).
type syncManifestsCommand struct{ baseCommand }

func (c *syncManifestsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	component := op.Spec.Component
	configChecksum := op.Spec.DesiredConfigChecksum
	pkiChecksum := op.Spec.DesiredPKIChecksum
	caChecksum := op.Spec.DesiredCAChecksum

	if configChecksum != "" {
		if err := writeExtraFiles(component, env.CPMSecretData, constants.ExtraFilesPath); err != nil {
			logger.Error("failed to write extra-files", log.Err(err))
			return reconcile.Result{}, err
		}
		if err := writeStaticPodManifest(component, env.CPMSecretData,
			configChecksum, pkiChecksum, caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to write manifest", log.Err(err))
			return reconcile.Result{}, err
		}
	} else {
		if err := updateChecksumAnnotations(component,
			pkiChecksum, caChecksum, op.CertRenewalID(), constants.ManifestsPath); err != nil {
			logger.Error("failed to update checksum annotations", log.Err(err))
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// waitPodReadyCommand waits for the static pod to become ready with the expected checksum annotations.
type waitPodReadyCommand struct {
	baseCommand
	pods podWaiter
}

func (c *waitPodReadyCommand) Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	env.State.SetReadyReason(constants.ReasonWaitingForPod,
		fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
			op.Spec.Component.PodComponentName(),
			shortChecksum(op.Spec.DesiredConfigChecksum),
			shortChecksum(op.Spec.DesiredPKIChecksum)))
	if err := env.FlushStatus(ctx); err != nil {
		return reconcile.Result{}, err
	}
	return c.pods.waitForPod(ctx, env.State, logger)
}

// syncHotReloadCommand writes config files that kube-apiserver picks up without restart.
type syncHotReloadCommand struct{ baseCommand }

func (c *syncHotReloadCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("writing hot-reload files")
	if err := writeHotReloadFiles(env.CPMSecretData, constants.ExtraFilesPath); err != nil {
		logger.Error("failed to write hot-reload files", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
