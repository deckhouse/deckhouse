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
	"os"
	"path/filepath"
	"time"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	etcdclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	etcdDataDir           = "/var/lib/etcd/member"
	etcdMemberListTimeout = 10 * time.Second
)

// etcdNeedsJoin checks if the etcd member needs to join the cluster:
//
//	/var/lib/etcd/member does not exist -> fresh node, needs join
//	exists but node not in member list -> orphan, needs join (caller must cleanup)
//	exists and node in member list -> normal update, no join needed
func etcdNeedsJoin(nodeName, pkiDir, kubeconfigDir string) (bool, error) {
	_, err := os.Stat(etcdDataDir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat etcd data dir: %w", err)
	}

	// Data dir exists — check if member is actually in cluster
	exists, err := checkEtcdMemberExists(nodeName, pkiDir, kubeconfigDir)
	if err != nil {
		return false, fmt.Errorf("check etcd membership: %w", err)
	}

	return !exists, nil
}

// cleanupEtcdDataDir removes stale etcd data directory for orphaned nodes.
func cleanupEtcdDataDir() error {
	return os.RemoveAll(etcdDataDir)
}

// checkEtcdMemberExists connects to the etcd cluster and checks if nodeName is in the member list.
func checkEtcdMemberExists(nodeName, pkiDir, kubeconfigDir string) (bool, error) {
	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")
	kubeClient, err := etcdclient.ClientSetFromFile(adminConfPath)
	if err != nil {
		return false, fmt.Errorf("create k8s client from admin.conf: %w", err)
	}

	etcdCli, err := etcdclient.New(kubeClient, pkiDir)
	if err != nil {
		return false, fmt.Errorf("create etcd client: %w", err)
	}
	defer etcdCli.Close()

	rawCli, ok := etcdCli.(*etcdclient.Client)
	if !ok {
		return false, fmt.Errorf("unexpected etcd client type: %T", etcdCli)
	}

	ctx, cancel := context.WithTimeout(context.Background(), etcdMemberListTimeout)
	defer cancel()

	resp, err := rawCli.Raw().MemberList(ctx)
	if err != nil {
		return false, fmt.Errorf("etcd member list: %w", err)
	}

	for _, m := range resp.Members {
		if m.Name == nodeName {
			return true, nil
		}
	}

	return false, nil
}

// ensureAdminKubeconfig creates admin.conf if it does not exist.
// CA files must be on disk (CA operation completed)
func ensureAdminKubeconfig(secretData map[string][]byte, pkiDir, kubeconfigDir string) error {
	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")
	if _, err := os.Stat(adminConfPath); err == nil {
		return nil // already exists
	}

	files := []kubeconfig.File{kubeconfig.Admin}

	algo := string(secretData[constants.SecretKeyEncryptionAlgorithm])
	if algo != "" {
		return kubeconfig.CreateKubeconfigFiles(files,
			kubeconfig.WithCertificatesDir(pkiDir),
			kubeconfig.WithOutDir(kubeconfigDir),
			kubeconfig.WithEncryptionAlgorithm(pkiconstants.EncryptionAlgorithmType(algo)),
		)
	}

	return kubeconfig.CreateKubeconfigFiles(files,
		kubeconfig.WithCertificatesDir(pkiDir),
		kubeconfig.WithOutDir(kubeconfigDir),
	)
}

// reconcileEtcdJoin handles etcd join for a fresh or orphaned node.
// Flow: ensureAdminKubeconfig -> prepare manifest with annotations -> JoinCluster -> waitForPod
func (r *Reconciler) reconcileEtcdJoin(
	ctx context.Context,
	op *controlplanev1alpha1.ControlPlaneOperation,
	secretData map[string][]byte,
	configChecksum, pkiChecksum, caChecksum string,
	logger *log.Logger,
) (reconcile.Result, error) {
	component := op.Spec.Component
	kubeconfigDir := kubeconfigDirPath()

	// Cleanup stale etcd data dir if it exists (orphaned node case)
	if _, err := os.Stat(etcdDataDir); err == nil {
		logger.Info("etcd join: cleaning up stale etcd data dir")
		if err := cleanupEtcdDataDir(); err != nil {
			logger.Error("failed to cleanup etcd data dir", log.Err(err))
			return reconcile.Result{}, r.setConditions(ctx, op,
				failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError,
					fmt.Sprintf("cleanup etcd data dir: %s", err)))
		}
	}

	logger.Info("etcd join: ensuring admin kubeconfig")
	if err := ensureAdminKubeconfig(secretData, constants.KubernetesPkiPath, kubeconfigDir); err != nil {
		logger.Error("failed to ensure admin kubeconfig", log.Err(err))
		return reconcile.Result{}, r.setConditions(ctx, op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError,
				fmt.Sprintf("ensure admin kubeconfig: %s", err)))
	}

	logger.Info("etcd join: preparing manifest")
	manifest, err := prepareManifestBytes(component, secretData, configChecksum, pkiChecksum, caChecksum)
	if err != nil {
		logger.Error("failed to prepare etcd manifest", log.Err(err))
		return reconcile.Result{}, r.setConditions(ctx, op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, err.Error()))
	}

	ip := os.Getenv("MY_IP")
	if ip == "" {
		return reconcile.Result{}, r.setConditions(ctx, op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError, "MY_IP env var is not set"))
	}

	logger.Info("etcd join: calling JoinCluster", slog.String("ip", ip))
	if err := etcd.JoinCluster(manifest, ip, r.nodeName,
		etcd.WithManifestDir(constants.ManifestsPath),
		etcd.WithCertificatesDir(constants.KubernetesPkiPath),
	); err != nil {
		logger.Error("etcd JoinCluster failed", log.Err(err))
		return reconcile.Result{}, r.setConditions(ctx, op,
			failedCondition(metav1.ConditionTrue, constants.ReasonManifestWriteError,
				fmt.Sprintf("etcd join cluster: %s", err)))
	}

	if err := r.setConditions(ctx, op,
		readyCondition(metav1.ConditionFalse, constants.ReasonWaitingForPod,
			fmt.Sprintf("waiting for etcd pod after join with config-checksum %s", shortChecksum(configChecksum))),
	); err != nil {
		return reconcile.Result{}, err
	}

	return r.waitForPod(ctx, op, configChecksum, pkiChecksum, caChecksum, logger)
}
