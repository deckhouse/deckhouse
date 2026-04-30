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
)

const (
	etcdDataDir           = "/var/lib/etcd/member"
	etcdMemberListTimeout = 10 * time.Second
)

// etcdNeedsJoin checks if the etcd member needs to join the cluster:
//
//	member in cluster (by name or peer URL) -> no join needed
//	/var/lib/etcd/member does not exist and not in cluster -> fresh node, needs join
//	data dir exists but not in cluster -> orphan, needs join (cleanup)
//
// must ensure admin.conf exists before calling (for ensureAdminKubeconfig).
func etcdNeedsJoin(node NodeIdentity, pkiDir, kubeconfigDir string) (bool, error) {
	peerURL := etcd.GetPeerURL(node.AdvertiseIP)
	exists, err := checkEtcdMemberExists(node.Name, peerURL, pkiDir, kubeconfigDir)
	if err != nil {
		return false, fmt.Errorf("check etcd membership: %w", err)
	}
	if exists {
		return false, nil
	}
	_, err = os.Stat(etcdDataDir)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat etcd data dir: %w", err)
	}

	// Data dir exists but member not in cluster - orphan node, needs rejoin
	return true, nil
}

// cleanupEtcdDataDir removes stale etcd data directory for orphaned nodes.
func cleanupEtcdDataDir() error {
	return os.RemoveAll(etcdDataDir)
}

// checkEtcdMemberExists connects to the etcd cluster and checks by name first, if name is empty (learner not yet started), falls back to peer URL match
func checkEtcdMemberExists(nodeName, peerURL, pkiDir, kubeconfigDir string) (bool, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), etcdMemberListTimeout)
	defer cancel()

	resp, err := etcdCli.Raw().MemberList(ctx)
	if err != nil {
		return false, fmt.Errorf("etcd member list: %w", err)
	}

	for _, m := range resp.Members {
		if m.Name == nodeName {
			return true, nil
		}
		if peerURL != "" {
			for _, u := range m.PeerURLs {
				if u == peerURL {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// ensureAdminKubeconfig creates admin.conf if it does not exist.
// CA files must be on disk (CA operation completed)
func ensureAdminKubeconfig(secretData map[string][]byte, pkiDir, kubeconfigDir, advertiseIP string) error {
	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")
	if _, err := os.Stat(adminConfPath); err == nil {
		return nil // already exists
	}

	files := []kubeconfig.File{kubeconfig.Admin}

	algo := string(secretData[constants.SecretKeyEncryptionAlgorithm])
	if algo != "" {
		_, err := kubeconfig.CreateKubeconfigFiles(files,
			kubeconfig.WithCertificatesDir(pkiDir),
			kubeconfig.WithOutDir(kubeconfigDir),
			kubeconfig.WithLocalAPIEndpoint(advertiseIP),
			kubeconfig.WithEncryptionAlgorithm(pkiconstants.EncryptionAlgorithmType(algo)),
		)
		return err
	}

	_, err := kubeconfig.CreateKubeconfigFiles(files,
		kubeconfig.WithCertificatesDir(pkiDir),
		kubeconfig.WithOutDir(kubeconfigDir),
		kubeconfig.WithLocalAPIEndpoint(advertiseIP),
	)
	return err
}

// reconcileEtcdJoin handles etcd join for a fresh or orphaned node.
// Precondition: caller must ensure admin.conf exists - see joinEtcdClusterStep.
func reconcileEtcdJoin(
	node NodeIdentity,
	component controlplanev1alpha1.OperationComponent,
	secretData map[string][]byte,
	annotations checksumAnnotations,
	logger *log.Logger,
) error {
	// Cleanup stale etcd data dir if it exists (orphaned node case)
	if _, err := os.Stat(etcdDataDir); err == nil {
		logger.Info("etcd join: cleaning up stale etcd data dir")
		if err := cleanupEtcdDataDir(); err != nil {
			logger.Error("failed to cleanup etcd data dir", log.Err(err))
			return fmt.Errorf("cleanup etcd data dir: %w", err)
		}
	}

	logger.Info("etcd join: preparing manifest")
	manifest, err := prepareManifestBytes(component, secretData, annotations)
	if err != nil {
		logger.Error("failed to prepare etcd manifest", log.Err(err))
		return fmt.Errorf("prepare etcd manifest: %w", err)
	}

	logger.Info("etcd join: calling JoinCluster", slog.String("ip", node.AdvertiseIP))
	if err := etcd.JoinCluster(manifest, node.AdvertiseIP, node.Name,
		etcd.WithManifestDir(constants.ManifestsPath),
		etcd.WithCertificatesDir(constants.KubernetesPkiPath),
	); err != nil {
		logger.Error("etcd JoinCluster failed", log.Err(err))
		return fmt.Errorf("etcd join cluster: %w", err)
	}

	return nil
}
