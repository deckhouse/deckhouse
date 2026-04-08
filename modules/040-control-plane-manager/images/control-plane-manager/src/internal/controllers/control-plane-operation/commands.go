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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type commandContext struct {
	r             *Reconciler
	op            *controlplanev1alpha1.ControlPlaneOperation
	cpmSecretData map[string][]byte
	pkiSecretData map[string][]byte
}

type PipelineCommand struct {
	Name        controlplanev1alpha1.CommandName
	ReadyReason string
	Exec        func(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error)
}

// commandRegistry maps each CommandName to PipelineCommand implementation.
var commandRegistry = map[controlplanev1alpha1.CommandName]PipelineCommand{
	controlplanev1alpha1.CommandSyncCA:           {controlplanev1alpha1.CommandSyncCA, constants.ReasonSyncingCA, execSyncCA},
	controlplanev1alpha1.CommandRenewPKICerts:    {controlplanev1alpha1.CommandRenewPKICerts, constants.ReasonRenewingPKI, execRenewPKICerts},
	controlplanev1alpha1.CommandRenewKubeconfigs: {controlplanev1alpha1.CommandRenewKubeconfigs, constants.ReasonRenewingKubeconfigs, execRenewKubeconfigs},
	controlplanev1alpha1.CommandSyncManifests:    {controlplanev1alpha1.CommandSyncManifests, constants.ReasonSyncingManifests, execSyncManifests},
	controlplanev1alpha1.CommandJoinEtcdCluster:  {controlplanev1alpha1.CommandJoinEtcdCluster, constants.ReasonJoiningEtcd, execJoinEtcdCluster},
	controlplanev1alpha1.CommandWaitPodReady:     {controlplanev1alpha1.CommandWaitPodReady, constants.ReasonWaitingForPod, execWaitPodReady},
	controlplanev1alpha1.CommandSyncHotReload:    {controlplanev1alpha1.CommandSyncHotReload, constants.ReasonSyncingHotReload, execSyncHotReload},
	controlplanev1alpha1.CommandCertObserve:      {controlplanev1alpha1.CommandCertObserve, constants.ReasonCertObserving, execCertObserve},
}

// resolveCommands looks up PipelineCommand entries for the given command names.
func resolveCommands(names []controlplanev1alpha1.CommandName) ([]PipelineCommand, error) {
	commands := make([]PipelineCommand, 0, len(names))
	for _, name := range names {
		cmd, ok := commandRegistry[name]
		if !ok {
			return nil, fmt.Errorf("unknown command: %s", name)
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

// execSyncCA installs CA files from the d8-pki secret to disk.
func execSyncCA(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("installing CA files from secret")
	if err := installCAsFromSecret(cc.pkiSecretData, constants.KubernetesPkiPath); err != nil {
		logger.Error("failed to install CAs", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// execRenewPKICerts renews leaf certificates for the component.
// No-op for KCM/Scheduler (certTree=nil).
func execRenewPKICerts(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	certTree := certTreeForComponent(cc.op.Spec.Component)
	if certTree != nil {
		logger.Info("renewing leaf certificates if needed")
		params := parsePKIParams(constants.KubernetesPkiPath, cc.cpmSecretData, cc.r.node)
		if err := renewCertsIfNeeded(params, certTree); err != nil {
			logger.Error("failed to renew certs", log.Err(err))
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// execRenewKubeconfigs renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
func execRenewKubeconfigs(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	component := cc.op.Spec.Component
	kubeconfigDir := cc.r.node.KubeconfigDir
	if err := renewKubeconfigsForComponent(component, cc.cpmSecretData, constants.KubernetesPkiPath, kubeconfigDir, cc.r.node.AdvertiseIP); err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, err
	}
	// dont return error if failed to update root kubeconfig symlink, maybe return reconcile.Result{}, err later.
	if needsRootKubeconfig(component) {
		if err := updateRootKubeconfig(kubeconfigDir, cc.r.node.HomeDir); err != nil {
			logger.Warn("failed to update root kubeconfig symlink", log.Err(err))
		}
	}

	return reconcile.Result{}, nil
}

// execJoinEtcdCluster checks if etcd needs to join the cluster and handles the full join flow.
// When join is needed, executes the full flow (cleanup -> ensureAdminKubeconfig -> JoinCluster -> waitForPod)
// No-op for non-etcd components. No-op if already in cluster.
func execJoinEtcdCluster(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	if cc.op.Spec.Component != controlplanev1alpha1.OperationComponentEtcd {
		return reconcile.Result{}, nil
	}

	kubeconfigDir := cc.r.node.KubeconfigDir

	// Ensure admin.conf exists before checking membership on fresh nodes (including etcd-arbiter)
	// admin.conf is absent until created here, ensureAdminKubeconfig is idempotent.
	if err := ensureAdminKubeconfig(cc.cpmSecretData, constants.KubernetesPkiPath, kubeconfigDir, cc.r.node.AdvertiseIP); err != nil {
		logger.Error("failed to ensure admin kubeconfig", log.Err(err))
		return reconcile.Result{}, fmt.Errorf("ensure admin kubeconfig: %w", err)
	}

	needsJoin, err := etcdNeedsJoin(cc.r.node, constants.KubernetesPkiPath, kubeconfigDir)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("check etcd join need: %w", err)
	}
	if !needsJoin {
		return reconcile.Result{}, nil
	}
	logger.Info("etcd needs join, executing join flow")
	return cc.r.reconcileEtcdJoin(cc.op, cc.cpmSecretData,
		cc.op.Spec.DesiredConfigChecksum, cc.op.Spec.DesiredPKIChecksum, cc.op.Spec.DesiredCAChecksum, logger)
}

// execSyncManifests writes the static pod manifest (or patches annotations for PKI-only updates).
func execSyncManifests(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	component := cc.op.Spec.Component
	configChecksum := cc.op.Spec.DesiredConfigChecksum
	pkiChecksum := cc.op.Spec.DesiredPKIChecksum
	caChecksum := cc.op.Spec.DesiredCAChecksum

	if configChecksum != "" {
		if err := writeExtraFiles(component, cc.cpmSecretData, constants.ExtraFilesPath); err != nil {
			logger.Error("failed to write extra-files", log.Err(err))
			return reconcile.Result{}, err
		}
		if err := writeStaticPodManifest(component, cc.cpmSecretData,
			configChecksum, pkiChecksum, caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to write manifest", log.Err(err))
			return reconcile.Result{}, err
		}
	} else {
		if err := updateChecksumAnnotations(component,
			pkiChecksum, caChecksum, cc.op.CertRenewalID(), constants.ManifestsPath); err != nil {
			logger.Error("failed to update checksum annotations", log.Err(err))
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// execWaitPodReady waits for the static pod to become ready with the expected checksum annotations.
func execWaitPodReady(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	if err := cc.r.setConditions(ctx, cc.op,
		readyCondition(metav1.ConditionFalse, constants.ReasonWaitingForPod,
			fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
				cc.op.Spec.Component.PodComponentName(),
				shortChecksum(cc.op.Spec.DesiredConfigChecksum),
				shortChecksum(cc.op.Spec.DesiredPKIChecksum))),
	); err != nil {
		return reconcile.Result{}, err
	}

	return cc.r.waitForPod(ctx, cc.op, logger)
}

// execSyncHotReload writes config files that kube-apiserver picks up without restart.
func execSyncHotReload(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("writing hot-reload files")
	if err := writeHotReloadFiles(cc.cpmSecretData, constants.ExtraFilesPath); err != nil {
		logger.Error("failed to write hot-reload files", log.Err(err))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
