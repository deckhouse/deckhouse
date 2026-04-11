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
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"

	"github.com/deckhouse/deckhouse/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Compile-time checks.
var (
	_ Command = (*backupCommand)(nil)
	_ Command = (*syncCACommand)(nil)
	_ Command = (*renewPKICertsCommand)(nil)
	_ Command = (*renewKubeconfigsCommand)(nil)
	_ Command = (*syncManifestsCommand)(nil)
	_ Command = (*joinEtcdClusterCommand)(nil)
	_ Command = (*waitPodReadyCommand)(nil)
	_ Command = (*syncHotReloadCommand)(nil)
	_ Command = (*certObserveCommand)(nil)
)

// Command is the interface each pipeline step must satisfy.
type Command interface {
	Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error)
}

// commandReadyReasons maps command names to their ready reason strings for status reporting.
var commandReadyReasons = map[controlplanev1alpha1.CommandName]string{
	controlplanev1alpha1.CommandBackup:           constants.ReasonCreatingBackup,
	controlplanev1alpha1.CommandSyncCA:           constants.ReasonSyncingCA,
	controlplanev1alpha1.CommandRenewPKICerts:    constants.ReasonRenewingPKI,
	controlplanev1alpha1.CommandRenewKubeconfigs: constants.ReasonRenewingKubeconfigs,
	controlplanev1alpha1.CommandSyncManifests:    constants.ReasonSyncingManifests,
	controlplanev1alpha1.CommandJoinEtcdCluster:  constants.ReasonJoiningEtcd,
	controlplanev1alpha1.CommandWaitPodReady:     constants.ReasonWaitingForPod,
	controlplanev1alpha1.CommandSyncHotReload:    constants.ReasonSyncingHotReload,
	controlplanev1alpha1.CommandCertObserve:      constants.ReasonCertObserving,
}

// defaultCommands returns a fresh command registry with all known commands.
// Reconciler-level deps (podWaiter) must be injected after construction.
func defaultCommands() map[controlplanev1alpha1.CommandName]Command {
	return map[controlplanev1alpha1.CommandName]Command{
		controlplanev1alpha1.CommandBackup:           &backupCommand{},
		controlplanev1alpha1.CommandSyncCA:           &syncCACommand{},
		controlplanev1alpha1.CommandRenewPKICerts:    &renewPKICertsCommand{},
		controlplanev1alpha1.CommandRenewKubeconfigs: &renewKubeconfigsCommand{},
		controlplanev1alpha1.CommandSyncManifests:    &syncManifestsCommand{},
		controlplanev1alpha1.CommandJoinEtcdCluster:  &joinEtcdClusterCommand{},
		controlplanev1alpha1.CommandWaitPodReady:     &waitPodReadyCommand{},
		controlplanev1alpha1.CommandSyncHotReload:    &syncHotReloadCommand{},
		controlplanev1alpha1.CommandCertObserve:      &certObserveCommand{},
	}
}

// resolveCommands validates that all command names exist in the registry.
func resolveCommands(registry map[controlplanev1alpha1.CommandName]Command, names []controlplanev1alpha1.CommandName) error {
	for _, name := range names {
		if _, ok := registry[name]; !ok {
			return fmt.Errorf("unknown command: %s", name)
		}
	}
	return nil
}

// syncCACommand installs CA files from the d8-pki secret to disk.
type syncCACommand struct{}

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
type renewPKICertsCommand struct{}

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
type renewKubeconfigsCommand struct{}

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
type joinEtcdClusterCommand struct{}

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
type syncManifestsCommand struct{}

func (c *syncManifestsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	component := op.Spec.Component
	configChecksum := op.Spec.DesiredConfigChecksum
	pkiChecksum := op.Spec.DesiredPKIChecksum
	caChecksum := op.Spec.DesiredCAChecksum
	var results []fileWriteResult

	if configChecksum != "" {
		extraResults, err := writeExtraFilesIfChanged(component, env.CPMSecretData, constants.ExtraFilesPath)
		if err != nil {
			logger.Error("failed to write extra-files", log.Err(err))
			return reconcile.Result{}, err
		}
		manifestResult, err := writeStaticPodManifestIfChanged(component, env.CPMSecretData,
			configChecksum, pkiChecksum, caChecksum, constants.ManifestsPath)
		if err != nil {
			logger.Error("failed to write manifest", log.Err(err))
			return reconcile.Result{}, err
		}
		results = append(results, extraResults...)
		results = append(results, manifestResult)
	} else {
		manifestResult, err := updateChecksumAnnotationsIfChanged(component,
			pkiChecksum, caChecksum, op.CertRenewalID(), constants.ManifestsPath)
		if err != nil {
			logger.Error("failed to update checksum annotations", log.Err(err))
			return reconcile.Result{}, err
		}
		results = append(results, manifestResult)
	}

	saveDiffResults(component, op.Name, results, logger)
	if !hasChangedFiles(results) {
		logger.Info("sync manifests no-op: desired content already on disk")
	}
	return reconcile.Result{}, nil
}

// waitPodReadyCommand waits for the static pod to become ready with the expected checksum annotations.
type waitPodReadyCommand struct {
	waitForPod func(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error)
}

func (c *waitPodReadyCommand) Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	env.State.SetReadyReason(constants.ReasonWaitingForPod,
		fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
			op.Spec.Component.PodComponentName(),
			checksum.ShortChecksum(op.Spec.DesiredConfigChecksum),
			checksum.ShortChecksum(op.Spec.DesiredPKIChecksum)))
	return c.waitForPod(ctx, env.State, logger)
}

// syncHotReloadCommand writes config files that kube-apiserver picks up without restart.
type syncHotReloadCommand struct{}

func (c *syncHotReloadCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("writing hot-reload files")
	results, err := writeHotReloadFilesIfChanged(env.CPMSecretData, constants.ExtraFilesPath)
	if err != nil {
		logger.Error("failed to write hot-reload files", log.Err(err))
		return reconcile.Result{}, err
	}
	saveDiffResults(controlplanev1alpha1.OperationComponentHotReload, env.State.Raw().Name, results, logger)
	if !hasChangedFiles(results) {
		logger.Info("sync hot-reload no-op: desired content already on disk")
	}
	return reconcile.Result{}, nil
}

func hasChangedFiles(results []fileWriteResult) bool {
	for i := range results {
		if results[i].Changed {
			return true
		}
	}
	return false
}
