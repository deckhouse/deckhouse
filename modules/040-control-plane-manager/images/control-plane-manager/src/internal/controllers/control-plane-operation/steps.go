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

type StepName string

const (
	StepSyncCA           StepName = "SyncCA"
	StepRenewPKI         StepName = "RenewPKICerts"
	StepRenewKubeconfigs StepName = "RenewKubeconfigs"
	StepSyncManifests    StepName = "SyncManifests"
	StepWaitPodReady     StepName = "WaitPodReady"
)

type stepContext struct {
	r              *Reconciler
	op             *controlplanev1alpha1.ControlPlaneOperation
	component      controlplanev1alpha1.OperationComponent
	cpmSecretData  map[string][]byte
	pkiSecretData  map[string][]byte
	configChecksum string
	pkiChecksum    string
	caChecksum     string
}

// PipelineStep is one unit of work in the reconciliation pipeline.
type PipelineStep struct {
	Name        StepName
	ReadyReason string
	Exec        func(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error)
}

// stepsForCommand returns the pipeline steps for a given command.
func stepsForCommand(cmd controlplanev1alpha1.OperationCommand) []PipelineStep {
	switch cmd {
	case controlplanev1alpha1.OperationCommandUpdate:
		return []PipelineStep{
			{StepSyncManifests, constants.ReasonSyncingManifests, execSyncManifests},
			{StepWaitPodReady, constants.ReasonWaitingForPod, execWaitPodReady},
		}
	case controlplanev1alpha1.OperationCommandUpdatePKI,
		controlplanev1alpha1.OperationCommandUpdateWithPKI:
		return []PipelineStep{
			{StepSyncCA, constants.ReasonSyncingCA, execSyncCA},
			{StepRenewPKI, constants.ReasonRenewingPKI, execRenewPKICerts},
			{StepRenewKubeconfigs, constants.ReasonRenewingKubeconfigs, execRenewKubeconfigs},
			{StepSyncManifests, constants.ReasonSyncingManifests, execSyncManifests},
			{StepWaitPodReady, constants.ReasonWaitingForPod, execWaitPodReady},
		}
	default:
		return nil
	}
}

// execSyncCA installs CA files from the d8-pki secret to disk.
func execSyncCA(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error) {
	logger.Info("installing CA files from secret")
	if err := installCAsFromSecret(sc.pkiSecretData, constants.KubernetesPkiPath); err != nil {
		logger.Error("failed to install CAs", log.Err(err))
		return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}
	return reconcile.Result{}, nil
}

// execRenewPKICerts renews leaf certificates for the component
// No-op for KCM/Scheduler (certTree=nil)
func execRenewPKICerts(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error) {
	certTree := certTreeForComponent(sc.component)
	if certTree != nil {
		logger.Info("renewing leaf certificates if needed")
		params := parsePKIParams(constants.KubernetesPkiPath, sc.cpmSecretData)
		if err := renewCertsIfNeeded(params, certTree); err != nil {
			logger.Error("failed to renew certs", log.Err(err))
			return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	}

	// Etcd join check for UpdateWithPKI: after certs are renewed, before kubeconfigs
	if sc.component == controlplanev1alpha1.OperationComponentEtcd && sc.configChecksum != "" {
		needsJoin, err := etcdNeedsJoin(sc.r.nodeName, constants.KubernetesPkiPath, kubeconfigDirPath())
		if err != nil {
			logger.Error("failed to check etcd join need", log.Err(err))
			return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
		}
		if needsJoin {
			logger.Info("etcd needs join, switching to join flow")
			return sc.r.reconcileEtcdJoin(ctx, sc.op, sc.cpmSecretData,
				sc.configChecksum, sc.pkiChecksum, sc.caChecksum, logger)
		}
	}

	return reconcile.Result{}, nil
}

// execRenewKubeconfigs renews kubeconfig files for the component.
// No-op for Etcd (no kubeconfigs). For KubeAPIServer also updates the root kubeconfig symlink.
func execRenewKubeconfigs(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error) {
	kubeconfigDir := kubeconfigDirPath()
	if err := renewKubeconfigsForComponent(sc.component, sc.cpmSecretData, constants.KubernetesPkiPath, kubeconfigDir); err != nil {
		logger.Error("failed to renew kubeconfigs", log.Err(err))
		return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}
	if needsRootKubeconfig(sc.component) {
		if err := updateRootKubeconfig(kubeconfigDir); err != nil {
			logger.Error("failed to update root kubeconfig symlink", log.Err(err))
		}
	}

	return reconcile.Result{}, nil
}

// execSyncManifests writes the static pod manifest (or patches annotations for PKI-only updates).
// For etcd with Update command, checks if join is needed before writing the manifest.
func execSyncManifests(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error) {
	// Etcd join check for Update command (no RenewPKI step ran before this).
	if sc.component == controlplanev1alpha1.OperationComponentEtcd &&
		sc.op.Spec.Command == controlplanev1alpha1.OperationCommandUpdate {
		needsJoin, err := etcdNeedsJoin(sc.r.nodeName, constants.KubernetesPkiPath, kubeconfigDirPath())
		if err != nil {
			logger.Error("failed to check etcd join need", log.Err(err))
			return reconcile.Result{RequeueAfter: requeueWaitPod}, nil
		}
		if needsJoin {
			logger.Info("etcd needs join, switching to join flow")
			return sc.r.reconcileEtcdJoin(ctx, sc.op, sc.cpmSecretData,
				sc.configChecksum, sc.pkiChecksum, sc.caChecksum, logger)
		}
	}

	if sc.configChecksum != "" {
		if err := writeExtraFiles(sc.component, sc.cpmSecretData, constants.ExtraFilesPath); err != nil {
			logger.Error("failed to write extra-files", log.Err(err))
			return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
		if err := writeStaticPodManifest(sc.component, sc.cpmSecretData,
			sc.configChecksum, sc.pkiChecksum, sc.caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to write manifest", log.Err(err))
			return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	} else {
		if err := updateChecksumAnnotations(sc.component,
			sc.pkiChecksum, sc.caChecksum, constants.ManifestsPath); err != nil {
			logger.Error("failed to update checksum annotations", log.Err(err))
			return reconcile.Result{}, sc.r.setConditions(ctx, sc.op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
		}
	}

	return reconcile.Result{}, nil
}

// execWaitPodReady waits for the static pod to become ready with the expected checksum annotations.
func execWaitPodReady(ctx context.Context, sc *stepContext, logger *log.Logger) (reconcile.Result, error) {
	if err := sc.r.setConditions(ctx, sc.op,
		readyCondition(metav1.ConditionFalse, constants.ReasonWaitingForPod,
			fmt.Sprintf("waiting for %s pod with config-checksum %s pki-checksum %s",
				sc.component.PodComponentName(),
				shortChecksum(sc.configChecksum),
				shortChecksum(sc.pkiChecksum))),
	); err != nil {
		return reconcile.Result{}, err
	}

	return sc.r.waitForPod(ctx, sc.op, sc.configChecksum, sc.pkiChecksum, sc.caChecksum, logger)
}
