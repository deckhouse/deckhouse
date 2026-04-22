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
	_ Command = (*certObserveCommand)(nil)
)

// Command is the interface each pipeline step must satisfy.
type Command interface {
	Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error)
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
	if err := installCAsFromSecret(env.Secrets.PKIData, constants.KubernetesPkiPath); err != nil {
		logger.Error("failed to install CAs", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// renewPKICertsCommand renews leaf certificates for the component.
// No-op for KCM/Scheduler (certTree=nil).
type renewPKICertsCommand struct{}

func (c *renewPKICertsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	renewResult := controlplanev1alpha1.CPOCommandResultNotRenewed
	certTree := componentDeps(env.State.Raw().Spec.Component).CertTree
	if certTree != nil {
		logger.Info("renewing leaf certificates if needed")
		params := parsePKIParams(constants.KubernetesPkiPath, env.Secrets.CPMData, env.Node)
		report, err := renewCertsIfNeeded(params, certTree)
		if err != nil {
			logger.Error("failed to renew certs", log.Err(err))
			return reconcile.Result{}, err
		}
		if hasRegeneratedCerts(report) {
			logger.Info("leaf certificates were regenerated")
			renewResult = controlplanev1alpha1.CPOCommandResultRenewed
		}
	}
	env.State.MarkCommandCompletedWithMessage(controlplanev1alpha1.CommandRenewPKICerts, renewResult)
	return reconcile.Result{}, nil
}

// renewKubeconfigsCommand renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
type renewKubeconfigsCommand struct{}

func (c *renewKubeconfigsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	component := env.State.Raw().Spec.Component
	kubeconfigDir := env.Node.KubeconfigDir
	renewResult := controlplanev1alpha1.CPOCommandResultNotRenewed
	kubeconfigsRenewed, err := renewKubeconfigsForComponent(component, env.Secrets.CPMData, constants.KubernetesPkiPath, kubeconfigDir, env.Node.AdvertiseIP)
	if err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, err
	}
	if kubeconfigsRenewed {
		logger.Info("kubeconfigs were regenerated")
		renewResult = controlplanev1alpha1.CPOCommandResultRenewed
	}
	// dont return error if failed to update root kubeconfig symlink, maybe return reconcile.Result{}, err later.
	if componentDeps(component).NeedsRootKubeconfig {
		if err := updateRootKubeconfig(kubeconfigDir, env.Node.HomeDir); err != nil {
			logger.Warn("failed to update root kubeconfig symlink", log.Err(err))
		}
	}
	env.State.MarkCommandCompletedWithMessage(controlplanev1alpha1.CommandRenewKubeconfigs, renewResult)
	return reconcile.Result{}, nil
}

// joinEtcdClusterCommand checks if etcd needs to join the cluster and handles the full join flow.
// No-op for non-etcd components.
type joinEtcdClusterCommand struct{}

func (c *joinEtcdClusterCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	if op.Spec.Component != controlplanev1alpha1.OperationComponentEtcd {
		return reconcile.Result{}, nil
	}

	kubeconfigDir := env.Node.KubeconfigDir

	// Ensure admin.conf exists before checking membership on fresh nodes (including etcd-arbiter)
	if err := ensureAdminKubeconfig(env.Secrets.CPMData, constants.KubernetesPkiPath, kubeconfigDir, env.Node.AdvertiseIP); err != nil {
		logger.Error("failed to ensure admin kubeconfig", log.Err(err))
		return reconcile.Result{}, fmt.Errorf("ensure admin kubeconfig: %w", err)
	}

	needsJoin, err := etcdNeedsJoin(env.Node, constants.KubernetesPkiPath, kubeconfigDir)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("check etcd join need: %w", err)
	}
	if !needsJoin {
		logger.Info("etcd already in cluster, syncing manifest to desired state")
		annotations := buildSyncManifestAnnotations(op)
		results, err := (&syncManifestsCommand{}).syncFullManifest(op.Spec.Component, env.Secrets.CPMData, annotations)
		if err != nil {
			logger.Error("failed to sync manifests for joined etcd member", log.Err(err))
			return reconcile.Result{}, err
		}
		saveDiffResults(op.Spec.Component, op.Name, results, logger)
		if !hasChangedFiles(results) {
			logger.Info("sync manifests no-op: desired content already on disk")
		}
		return reconcile.Result{}, nil
	}
	logger.Info("etcd needs join, executing join flow")
	return reconcileEtcdJoin(env.Node, op.Spec.Component, env.Secrets.CPMData, checksumAnnotationsFromSpec(op.Spec), logger)
}

// syncManifestsCommand writes the static pod manifest (or patches annotations for PKI-only updates).
type syncManifestsCommand struct{}

func (c *syncManifestsCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	component := op.Spec.Component
	annotations := buildSyncManifestAnnotations(op)
	var (
		results []fileWriteResult
		err     error
	)

	if annotations.ConfigChecksum != "" {
		results, err = c.syncFullManifest(component, env.Secrets.CPMData, annotations)
	} else {
		results, err = c.syncAnnotationsOnly(component, annotations)
	}
	if err != nil {
		logger.Error("failed to sync manifests", log.Err(err))
		return reconcile.Result{}, err
	}

	saveDiffResults(component, op.Name, results, logger)
	if !hasChangedFiles(results) {
		logger.Info("sync manifests no-op: desired content already on disk")
	}
	return reconcile.Result{}, nil
}

func (c *syncManifestsCommand) syncFullManifest(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, annotations checksumAnnotations) ([]fileWriteResult, error) {
	extraResults, err := writeExtraFilesIfChanged(component, secretData, constants.ExtraFilesPath)
	if err != nil {
		return nil, fmt.Errorf("write extra-files: %w", err)
	}

	manifestResult, err := writeStaticPodManifestIfChanged(component, secretData, annotations, constants.ManifestsPath)
	if err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}

	results := append(extraResults, manifestResult)
	results = append(results, removeStaleExtraFiles(component, secretData, constants.ExtraFilesPath)...)
	return results, nil
}

func (c *syncManifestsCommand) syncAnnotationsOnly(component controlplanev1alpha1.OperationComponent, annotations checksumAnnotations) ([]fileWriteResult, error) {
	manifestResult, err := updateChecksumAnnotationsIfChanged(component, annotations, constants.ManifestsPath)
	if err != nil {
		return nil, fmt.Errorf("update checksum annotations: %w", err)
	}
	return []fileWriteResult{manifestResult}, nil
}

// waitPodReadyCommand waits for the static pod to become ready with the expected checksum annotations.
type waitPodReadyCommand struct {
	waitForPod func(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error)
}

func (c *waitPodReadyCommand) Execute(ctx context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	env.State.MarkCommandInProgressWithMessage(controlplanev1alpha1.CommandWaitPodReady,
		fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
			op.Spec.Component.PodComponentName(),
			checksum.ShortChecksum(op.Spec.DesiredConfigChecksum),
			checksum.ShortChecksum(op.Spec.DesiredPKIChecksum)))
	return c.waitForPod(ctx, env.State, logger)
}

func hasChangedFiles(results []fileWriteResult) bool {
	for i := range results {
		if results[i].Changed {
			return true
		}
	}
	return false
}

// certObserveCommand collects certificate expiration dates from disk and writes them to CPO status.
type certObserveCommand struct{}

func (c *certObserveCommand) Execute(_ context.Context, env *CommandEnv, logger *log.Logger) (reconcile.Result, error) {
	kubeconfigDir := env.Node.KubeconfigDir
	component := env.State.Raw().Spec.Component
	observedState, ok := observeCertExpirationsForStaticPod(component, kubeconfigDir, logger)
	if !ok {
		logger.Warn("CertObserve skipped: not a static pod component")
		return reconcile.Result{}, nil
	}

	if len(observedState.CertificatesExpirationDate) == 0 {
		logger.Info("observed certificate expiration", slog.Int("certificates", 0))
		return reconcile.Result{}, nil
	}

	env.State.SetObservedState(&observedState)
	logger.Info("observed certificate expiration", slog.Int("certificates", len(observedState.CertificatesExpirationDate)))
	return reconcile.Result{}, nil
}
