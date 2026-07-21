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

	pkiconstants "github.com/deckhouse/deckhouse/go_lib/controlplane/constants"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/etcd"
	etcdclient "github.com/deckhouse/deckhouse/go_lib/controlplane/etcd/client"
	"github.com/deckhouse/deckhouse/go_lib/controlplane/kubeconfig"
	"github.com/deckhouse/deckhouse/pkg/log"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

const (
	etcdDataDir           = "/var/lib/etcd/member"
	etcdMemberListTimeout = 10 * time.Second
)

type etcdJoinState int

const (
	// etcdJoined: our member is registered and local etcd has bootstrapped. etcd ignores --initial-cluster on restart, so a plain manifest sync + idempotent promote is safe.
	etcdJoined etcdJoinState = iota
	// etcdNeedsJoin: our member must (re)run the idempotent join flow, the only path that writes a manifest with the full --initial-cluster.
	// Covers both a fresh node and an interrupted join (member added but local etcd never bootstrapped); both lead to the same idempotent JoinCluster.
	// The data dir is NOT wiped: an interrupted-join etcd may already be starting.
	etcdNeedsJoin
	// etcdOrphan: our member is not registered but a stale local data dir exists. Wipe it and rejoin fresh.
	etcdOrphan
	// etcdNameConflict: a member carries our node name but none of its peer URLs is ours (e.g. the node was recreated with a new IP).
	// Joining would add a second member and leave the old, unreachable voter in place (the removal hook keeps it by name), changing quorum requirements.
	// Fail closed and require an explicit member replacement.
	etcdNameConflict
)

type etcdMemberInfo struct {
	Name      string
	PeerURLs  []string
	IsLearner bool
}

type etcdClassification struct {
	state     etcdJoinState
	isLearner bool
}

// classifyEtcd is the pure decision given the current members, whether the local data dir exists, and this node identity (name + peer URL).
//
// A member with our node name but none of its peer URLs equal to ours is a distinct, conflicting member (IP change): this takes priority over a successful peer-URL match,
// because promoting/joining while a stale same-name voter lingers would strand quorum.
func classifyEtcd(members []etcdMemberInfo, dataDirPresent bool, nodeName, peerURL string) etcdClassification {
	var ourMember *etcdMemberInfo
	nameConflict := false
	for i := range members {
		m := &members[i]
		if memberHasPeerURL(m, peerURL) {
			ourMember = m
			continue
		}
		if nodeName != "" && m.Name == nodeName {
			nameConflict = true
		}
	}

	switch {
	case nameConflict:
		return etcdClassification{state: etcdNameConflict}
	case ourMember != nil && dataDirPresent:
		return etcdClassification{state: etcdJoined, isLearner: ourMember.IsLearner}
	case ourMember != nil:
		return etcdClassification{state: etcdNeedsJoin, isLearner: ourMember.IsLearner} // interrupted join
	case dataDirPresent:
		return etcdClassification{state: etcdOrphan}
	default:
		return etcdClassification{state: etcdNeedsJoin} // fresh
	}
}

func memberHasPeerURL(m *etcdMemberInfo, peerURL string) bool {
	for _, u := range m.PeerURLs {
		if u == peerURL {
			return true
		}
	}
	return false
}

// classifyEtcdState fetches the membership snapshot and data-dir state, then runs the pure classifier.
//
// must ensure admin.conf exists before calling (for ensureAdminKubeconfig).
func classifyEtcdState(node NodeIdentity, pkiDir, kubeconfigDir string) (etcdClassification, error) {
	members, err := snapshotEtcdMembers(pkiDir, kubeconfigDir)
	if err != nil {
		return etcdClassification{}, err
	}
	dataDirPresent, err := etcdDataDirExists()
	if err != nil {
		return etcdClassification{}, fmt.Errorf("stat etcd data dir: %w", err)
	}
	return classifyEtcd(members, dataDirPresent, node.Name, etcd.GetPeerURL(node.AdvertiseIP)), nil
}

// snapshotEtcdMembers returns the current etcd membership as plain etcdMemberInfo values.
func snapshotEtcdMembers(pkiDir, kubeconfigDir string) ([]etcdMemberInfo, error) {
	adminConfPath := filepath.Join(kubeconfigDir, "admin.conf")
	kubeClient, err := etcdclient.ClientSetFromFile(adminConfPath)
	if err != nil {
		return nil, fmt.Errorf("create k8s client from admin.conf: %w", err)
	}
	etcdCli, err := etcdclient.New(kubeClient, pkiDir)
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}
	defer etcdCli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdMemberListTimeout)
	defer cancel()
	resp, err := etcdCli.Raw().MemberList(ctx)
	if err != nil {
		return nil, fmt.Errorf("etcd member list: %w", err)
	}

	members := make([]etcdMemberInfo, 0, len(resp.Members))
	for _, m := range resp.Members {
		members = append(members, etcdMemberInfo{
			Name:      m.Name,
			PeerURLs:  m.PeerURLs,
			IsLearner: m.IsLearner,
		})
	}
	return members, nil
}

// etcdDataDirExists reports whether the local etcd data directory is present, meaning etcd has bootstrapped its storage at least once (and ignores --initial-cluster on restart).
func etcdDataDirExists() (bool, error) {
	_, err := os.Stat(etcdDataDir)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// cleanupEtcdDataDir removes stale etcd data directory for orphaned nodes.
func cleanupEtcdDataDir() error {
	return os.RemoveAll(etcdDataDir)
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

// reconcileEtcdJoin runs the idempotent etcd join flow (etcd.JoinCluster).
// It does NOT touch the data dir: any stale-data cleanup is the caller's responsibility, done only for the explicitly classified etcdOrphan state (see joinEtcdClusterStep).
// Precondition: caller must ensure admin.conf exists - see joinEtcdClusterStep.
func reconcileEtcdJoin(
	node NodeIdentity,
	component controlplanev1alpha1.OperationComponent,
	secretData map[string][]byte,
	annotations checksumAnnotations,
	logger *log.Logger,
) error {
	logger.Info("etcd join: preparing manifest")
	manifest, err := prepareManifestWithOverrides(component, secretData, annotations, node)
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
