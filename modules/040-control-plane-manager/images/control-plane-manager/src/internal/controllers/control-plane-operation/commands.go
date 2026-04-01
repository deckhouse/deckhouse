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
	r              *Reconciler
	op             *controlplanev1alpha1.ControlPlaneOperation
	component      controlplanev1alpha1.OperationComponent
	cpmSecretData  map[string][]byte
	pkiSecretData  map[string][]byte
	configChecksum string
	pkiChecksum    string
	caChecksum     string
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
		return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}
	return reconcile.Result{}, nil
}

// execRenewPKICerts renews leaf certificates for the component.
// No-op for KCM/Scheduler (certTree=nil).
func execRenewPKICerts(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	certTree := certTreeForComponent(cc.component)
	if certTree != nil {
		logger.Info("renewing leaf certificates if needed")
		params := parsePKIParams(constants.KubernetesPkiPath, cc.cpmSecretData)
		if err := renewCertsIfNeeded(params, certTree); err != nil {
			logger.Error("failed to renew certs", log.Err(err))
			return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	}

	return reconcile.Result{}, nil
}

// execRenewKubeconfigs renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
func execRenewKubeconfigs(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	kubeconfigDir := kubeconfigDirPath()
	if err := renewKubeconfigsForComponent(cc.component, cc.cpmSecretData, constants.KubernetesPkiPath, kubeconfigDir); err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}
	if needsRootKubeconfig(cc.component) {
		if err := updateRootKubeconfig(kubeconfigDir); err != nil {
			logger.Error("failed to update root kubeconfig symlink", log.Err(err))
		}
	}

	return reconcile.Result{}, nil
}

// execJoinEtcdCluster checks if etcd needs to join the cluster and handles the full join flow.
// When join is needed, executes the full flow (cleanup -> ensureAdminKubeconfig -> JoinCluster -> waitForPod)
// No-op for non-etcd components. No-op if already in cluster.
func execJoinEtcdCluster(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	if cc.component != controlplanev1alpha1.OperationComponentEtcd {
		return reconcile.Result{}, nil
	}
	needsJoin, err := etcdNeedsJoin(cc.r.nodeName, constants.KubernetesPkiPath, kubeconfigDirPath())
	if err != nil {
		logger.Error("failed to check etcd join need", log.Err(err))
		return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
	}
	if !needsJoin {
		return reconcile.Result{}, nil
	}
	logger.Info("etcd needs join, executing join flow")
	return cc.r.reconcileEtcdJoin(ctx, cc.op, cc.cpmSecretData,
		cc.configChecksum, cc.pkiChecksum, cc.caChecksum, logger)
}

// execSyncManifests writes the static pod manifest (or patches annotations for PKI-only updates).
func execSyncManifests(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	if cc.configChecksum != "" {
		if err := writeExtraFiles(cc.component, cc.cpmSecretData, constants.ExtraFilesPath); err != nil {
			logger.Error("failed to write extra-files", log.Err(err))
			return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
		if err := writeStaticPodManifest(cc.component, cc.cpmSecretData,
			cc.configChecksum, cc.pkiChecksum, cc.caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to write manifest", log.Err(err))
			return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	} else {
		if err := updateChecksumAnnotations(cc.component,
			cc.pkiChecksum, cc.caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to update checksum annotations", log.Err(err))
			return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	}

	return reconcile.Result{}, nil
}

// execWaitPodReady waits for the static pod to become ready with the expected checksum annotations.
func execWaitPodReady(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	if err := cc.r.setConditions(ctx, cc.op,
		readyCondition(metav1.ConditionFalse, constants.ReasonWaitingForPod,
			fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
				cc.component.PodComponentName(),
				shortChecksum(cc.configChecksum),
				shortChecksum(cc.pkiChecksum))),
	); err != nil {
		return reconcile.Result{}, err
	}

	return cc.r.waitForPod(ctx, cc.op, cc.configChecksum, cc.pkiChecksum, cc.caChecksum, logger)
}

// execSyncHotReload writes config files that kube-apiserver picks up without restart.
func execSyncHotReload(ctx context.Context, cc *commandContext, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("writing hot-reload files")
	if err := writeHotReloadFiles(cc.cpmSecretData, constants.ExtraFilesPath); err != nil {
		logger.Error("failed to write hot-reload files", log.Err(err))
		return reconcile.Result{}, cc.r.setConditions(ctx, cc.op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}
	return reconcile.Result{}, nil
}
