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
	_ Step = (*backupStep)(nil)
	_ Step = (*syncCAStep)(nil)
	_ Step = (*renewPKICertsStep)(nil)
	_ Step = (*renewKubeconfigsStep)(nil)
	_ Step = (*syncManifestsStep)(nil)
	_ Step = (*joinEtcdClusterStep)(nil)
	_ Step = (*waitPodReadyStep)(nil)
	_ Step = (*certObserveStep)(nil)
)

// Step is the interface each pipeline step must satisfy.
type Step interface {
	Execute(ctx context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error)
}

// defaultSteps returns a fresh step registry with all known steps.
// Reconciler-level deps (podWaiter) must be injected after construction.
func defaultSteps() map[controlplanev1alpha1.StepName]Step {
	return map[controlplanev1alpha1.StepName]Step{
		controlplanev1alpha1.StepBackup:           &backupStep{},
		controlplanev1alpha1.StepSyncCA:           &syncCAStep{},
		controlplanev1alpha1.StepRenewPKICerts:    &renewPKICertsStep{},
		controlplanev1alpha1.StepRenewKubeconfigs: &renewKubeconfigsStep{},
		controlplanev1alpha1.StepSyncManifests:    &syncManifestsStep{},
		controlplanev1alpha1.StepJoinEtcdCluster:  &joinEtcdClusterStep{},
		controlplanev1alpha1.StepWaitPodReady:     &waitPodReadyStep{},
		controlplanev1alpha1.StepCertObserve:      &certObserveStep{},
	}
}

// resolveSteps validates that all step names exist in the registry.
func resolveSteps(registry map[controlplanev1alpha1.StepName]Step, names []controlplanev1alpha1.StepName) error {
	for _, name := range names {
		if _, ok := registry[name]; !ok {
			return fmt.Errorf("unknown step: %s", name)
		}
	}
	return nil
}

// syncCAStep installs CA files from the d8-pki secret to disk.
type syncCAStep struct{}

func (c *syncCAStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("installing CA files from secret")
	if err := installCAsFromSecret(env.Secrets.PKIData, constants.KubernetesPkiPath); err != nil {
		logger.Error("failed to install CAs", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// renewPKICertsStep renews leaf certificates for the component.
// No-op for KCM/Scheduler (certTree=nil).
type renewPKICertsStep struct{}

func (c *renewPKICertsStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
	renewResult := controlplanev1alpha1.CPOStepResultNotRenewed
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
			renewResult = controlplanev1alpha1.CPOStepResultRenewed
		}
	}
	env.State.MarkStepCompletedWithMessage(controlplanev1alpha1.StepRenewPKICerts, renewResult)
	return reconcile.Result{}, nil
}

// renewKubeconfigsStep renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
type renewKubeconfigsStep struct{}

func (c *renewKubeconfigsStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
	component := env.State.Raw().Spec.Component
	kubeconfigDir := env.Node.KubeconfigDir
	renewResult := controlplanev1alpha1.CPOStepResultNotRenewed
	kubeconfigsRenewed, err := renewKubeconfigsForComponent(component, env.Secrets.CPMData, constants.KubernetesPkiPath, kubeconfigDir, env.Node.AdvertiseIP)
	if err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, err
	}
	if kubeconfigsRenewed {
		logger.Info("kubeconfigs were regenerated")
		renewResult = controlplanev1alpha1.CPOStepResultRenewed
	}
	// dont return error if failed to update root kubeconfig symlink, maybe return reconcile.Result{}, err later.
	if componentDeps(component).NeedsRootKubeconfig {
		if err := updateRootKubeconfig(kubeconfigDir, env.Node.HomeDir); err != nil {
			logger.Warn("failed to update root kubeconfig symlink", log.Err(err))
		}
	}
	env.State.MarkStepCompletedWithMessage(controlplanev1alpha1.StepRenewKubeconfigs, renewResult)
	return reconcile.Result{}, nil
}

// joinEtcdClusterStep checks if etcd needs to join the cluster and handles the full join flow.
// No-op for non-etcd components.
type joinEtcdClusterStep struct{}

func (c *joinEtcdClusterStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
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
		results, err := (&syncManifestsStep{}).syncFullManifest(op.Spec.Component, env.Secrets.CPMData, annotations)
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

// syncManifestsStep writes the static pod manifest (or patches annotations for PKI-only updates).
type syncManifestsStep struct{}

func (c *syncManifestsStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
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

func (c *syncManifestsStep) syncFullManifest(component controlplanev1alpha1.OperationComponent, secretData map[string][]byte, annotations checksumAnnotations) ([]fileWriteResult, error) {
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

func (c *syncManifestsStep) syncAnnotationsOnly(component controlplanev1alpha1.OperationComponent, annotations checksumAnnotations) ([]fileWriteResult, error) {
	manifestResult, err := updateChecksumAnnotationsIfChanged(component, annotations, constants.ManifestsPath)
	if err != nil {
		return nil, fmt.Errorf("update checksum annotations: %w", err)
	}
	return []fileWriteResult{manifestResult}, nil
}

// waitPodReadyStep waits for the static pod to become ready with the expected checksum annotations.
type waitPodReadyStep struct {
	waitForPod func(ctx context.Context, state *controlplanev1alpha1.OperationState, logger *log.Logger) (reconcile.Result, error)
}

func (c *waitPodReadyStep) Execute(ctx context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
	op := env.State.Raw()
	env.State.MarkStepInProgressWithMessage(controlplanev1alpha1.StepWaitPodReady,
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

// certObserveStep collects certificate expiration dates from disk and writes them to CPO status.
type certObserveStep struct{}

func (c *certObserveStep) Execute(_ context.Context, env *StepEnv, logger *log.Logger) (reconcile.Result, error) {
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
