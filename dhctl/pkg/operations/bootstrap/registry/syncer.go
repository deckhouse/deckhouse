// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

// CacheFillParams carries all the inputs needed to fill the in-cluster cache from
// the on-node bootstrap seed registry and finalize the bootstrap sequence.
//
// Design notes (new-arch bootstrap):
//   - The source is the on-node bootstrap seed (127.0.0.1:5010), a raw-process
//     registry started on the first master during early bootstrap. No SSH tunnel
//     is involved: both the seed and the cache leader are reachable from the master
//     directly (cache leader via CoreDNS once kubeadm init completes).
//   - registry-syncer is installed at /opt/deckhouse/bin/registry-syncer on the
//     master by the registry-syncer DaemonSet's install script (RPP); dhctl execs
//     it remotely over SSH rather than re-uploading the binary.
//   - Finalize order: WaitForCacheAndAgentReady → FillCacheFromSeed →
//     WaitForRegistryInitialization (removes registry-init secret). Once the cache
//     is filled, agent+cache serve all containerd mirror traffic.
//
// TODO(e2e-gate): The full flow (wait for cache+agent ready → syncer exec →
// finalize) is exercised by the air-gap bootstrap e2e:
//   modules/038-registry/e2e/bootstrap/air-gap/
// Local validation is build-only (go build/vet + unit tests for pure helpers).

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const (
	// syncerBinPath is the on-master path where the registry-syncer binary is
	// installed by the syncer DaemonSet's install script (see
	// modules/038-registry/images/syncer/scripts/install).
	syncerBinPath = "/opt/deckhouse/bin/registry-syncer"

	// syncerConfigPath is the temporary config file path written on the master
	// before invoking the syncer. Chosen under /tmp so no special permissions
	// are needed; removed after the sync completes.
	syncerConfigPath = "/tmp/dhctl-registry-syncer-config.yaml"

	// cacheFillSyncerSource is the on-node raw-process bootstrap seed, local on
	// the first master (no tunnel). The seed serves https with the module CA and
	// requires RO auth.
	cacheFillSyncerSource = "127.0.0.1:5010"

	// cacheFillDestination is the in-cluster cache leader Service. Resolvable on
	// the master once k8s is up and CoreDNS is running.
	cacheFillDestination = "registry-cache-leader.d8-system.svc:5001"

	// cacheReadyPollAttempts / cacheReadyPollWait define the readiness-wait
	// budget for the registry-cache StatefulSet + registry-agent DaemonSet.
	// 150 attempts × 10 s = 25 minutes, consistent with other deckhouse waits.
	cacheReadyPollAttempts = 150
	cacheReadyPollWait     = 10 * time.Second

	// syncerExecTimeout is the wall-clock budget for a single syncer invocation.
	// A full bundle sync of a typical Deckhouse release is expected to complete
	// well within 20 minutes; the timeout is generous to handle slow links.
	syncerExecTimeout = 30 * time.Minute
)

// SyncerConfig is the mirror-mode configuration for registry-syncer.
// It intentionally mirrors modules/038-registry/images/syncer/app/pkg/config/config.go
// so that the on-disk YAML is valid for the syncer binary.
//
// We do NOT import the syncer package directly because it lives in a separate
// Go module (modules/038-registry/images/syncer/app/) and is not vendored into
// dhctl. The struct duplication is intentional and is documented here to keep
// the two in sync when the syncer config schema changes.
type SyncerConfig struct {
	Src   SyncerRegistry `json:"source"`
	Dest  SyncerRegistry `json:"destination"`
	Prune bool           `json:"prune,omitempty"`
}

// SyncerRegistry mirrors config.Registry in the syncer package.
type SyncerRegistry struct {
	Address string      `json:"address"`
	User    *SyncerUser `json:"user,omitempty"`
	CA      string      `json:"ca,omitempty"`
}

// SyncerUser mirrors config.User in the syncer package.
type SyncerUser struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// BuildCacheFillSyncerConfig builds the one-shot syncer config that copies the
// on-node bootstrap seed (local on the master) into the in-cluster cache leader.
// Source = the seed (https, module CA, RO creds); dest = the cache leader (https,
// module CA, RW creds). Additive (prune=false): the bootstrap fill never deletes.
func BuildCacheFillSyncerConfig(caBundle, roUserName, roUserPassword, rwUserName, rwUserPassword string) SyncerConfig {
	return SyncerConfig{
		Src: SyncerRegistry{
			Address: cacheFillSyncerSource,
			CA:      caBundle,
			User: &SyncerUser{
				Name:     roUserName,
				Password: roUserPassword,
			},
		},
		Dest: SyncerRegistry{
			Address: cacheFillDestination,
			CA:      caBundle,
			User: &SyncerUser{
				Name:     rwUserName,
				Password: rwUserPassword,
			},
		},
		Prune: false, // additive — never delete images during bootstrap fill
	}
}

// CacheFillParams holds the runtime dependencies for FillCacheFromSeed.
type CacheFillParams struct {
	// NodeInterface is the SSH node connection to the first master.
	// Used to upload the config file and exec the syncer binary.
	NodeInterface libcon.Interface

	// PKICA is the module CA (PEM): trusts both the seed source and the cache dest.
	PKICA string

	// Seed source RO credentials (the seed requires token auth).
	PKIROUserName     string
	PKIROUserPassword string

	// Cache destination RW credentials.
	PKIRWUserName     string
	PKIRWUserPassword string
}

// FillCacheFromSeed copies the on-node bootstrap seed into the in-cluster cache
// leader by exec'ing registry-syncer on the first master (over SSH, no tunnel —
// both endpoints are reachable from the master). Idempotent/retriable.
//
//  1. Build the syncer mirror config (source = seed, dest = cache leader).
//  2. Upload the config file to the master via SCP.
//  3. Run registry-syncer (one-shot, blocking) via SSH exec with sudo.
//  4. Remove the temp config file.
//
// The caller is responsible for:
//   - Calling WaitForRegistryInitialization after this function succeeds.
//
// TODO(e2e-gate): Exercised by modules/038-registry/e2e/bootstrap/air-gap/.
// This is a best-effort skeleton until the air-gap e2e provides live coverage.
// The implementation uses the same SSH exec facilities as readRemoteFile and the
// bashible runner — Command + File.UploadBytes.
func FillCacheFromSeed(ctx context.Context, params CacheFillParams) error {
	cfg := BuildCacheFillSyncerConfig(
		params.PKICA,
		params.PKIROUserName, params.PKIROUserPassword,
		params.PKIRWUserName, params.PKIRWUserPassword,
	)

	cfgBytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal syncer config: %w", err)
	}

	// Step 1: Upload the syncer config to the master.
	if err := params.NodeInterface.File().UploadBytes(ctx, cfgBytes, syncerConfigPath); err != nil {
		return fmt.Errorf("upload syncer config to master: %w", err)
	}

	// Step 2: Execute the syncer binary on the master (via sudo).
	// The binary is already installed at syncerBinPath by the syncer DaemonSet.
	// We run it synchronously; it exits 0 on success, non-zero on failure.
	cmd := params.NodeInterface.Command(syncerBinPath, syncerConfigPath)
	cmd.Sudo(ctx)
	cmd.WithTimeout(syncerExecTimeout)
	logger := log.GetDefaultLogger()
	cmd.WithStdoutHandler(func(line string) {
		logger.LogInfoLn("registry-syncer:", line)
	})
	cmd.WithStderrHandler(func(line string) {
		logger.LogWarnLn("registry-syncer stderr:", line)
	})

	if err := cmd.Run(ctx); err != nil {
		// Best-effort cleanup of the temp config even on failure.
		_ = cleanupSyncerConfig(ctx, params.NodeInterface)
		return fmt.Errorf("run registry-syncer on master: %w", err)
	}

	// Step 3: Remove the temp config (contains credentials).
	if err := cleanupSyncerConfig(ctx, params.NodeInterface); err != nil {
		// Log-only: failure to clean up should not abort the bootstrap.
		// The temp file holds the registry RW password; log the error so the
		// operator is aware and can remove it manually if needed.
		log.GetDefaultLogger().LogWarnLn("registry-syncer: failed to remove temp config file:", err)
	}

	return nil
}

// CacheFillParamsFromInitSecret reads the registry-init secret on the master
// (data.config = base64 YAML initsecret.Config) and builds CacheFillParams: the
// module CA + the RO (seed source) and RW (cache dest) credentials. These are the
// exact creds the seed and cache were configured with at bootstrap.
//
// config.Registry exposes no bootstrap-cred accessors (the init creds live in the
// in-cluster registry-init secret, not in the dhctl install config), so we fetch
// them from the cluster at finalize time — the most robust source, since the seed
// and cache were brought up with exactly these credentials. node is carried into
// CacheFillParams for the subsequent on-node syncer exec.
func CacheFillParamsFromInitSecret(ctx context.Context, kubeCl *client.KubernetesClient, node libcon.Interface) (CacheFillParams, error) {
	secret, err := kubeCl.CoreV1().Secrets("d8-system").Get(ctx, "registry-init", metav1.GetOptions{})
	if err != nil {
		return CacheFillParams{}, fmt.Errorf("get secret registry-init: %w", err)
	}
	// Secret.Data values are already base64-decoded by the typed client.
	var cfg initsecret.Config
	if err := yaml.Unmarshal(secret.Data["config"], &cfg); err != nil {
		return CacheFillParams{}, fmt.Errorf("parse registry-init config: %w", err)
	}
	return CacheFillParams{
		NodeInterface:     node,
		PKICA:             cfg.CA.Cert,
		PKIROUserName:     cfg.ROUser.Name,
		PKIROUserPassword: cfg.ROUser.Password,
		PKIRWUserName:     cfg.RWUser.Name,
		PKIRWUserPassword: cfg.RWUser.Password,
	}, nil
}

// VerifyCacheNonEmpty re-runs FillCacheFromSeed's syncer once more as the
// non-empty signal: a clean (exit-0) sync after the bring-up fill means the cache
// leader accepted the seed's repository set. Cheap and idempotent (additive).
func VerifyCacheNonEmpty(ctx context.Context, params CacheFillParams) error {
	return FillCacheFromSeed(ctx, params)
}

// DeleteBootstrapSecret removes the registry-bootstrap secret on the master so the
// module drops the seed mirror from RegistryConfig and the agent re-renders
// containerd to [agent, cache] only. Idempotent (NotFound tolerated).
func DeleteBootstrapSecret(ctx context.Context, kubeCl *client.KubernetesClient) error {
	err := kubeCl.CoreV1().Secrets("d8-system").Delete(ctx, "registry-bootstrap", metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete registry-bootstrap secret: %w", err)
	}
	return nil
}

// TeardownSeed stops the on-node seed processes and removes the seed store. The
// agent owns containerd registry.d (the .managed-by-agent marker), so there is no
// registry.d drop-in to remove here. Idempotent: a re-run is harmless.
func TeardownSeed(ctx context.Context, node libcon.Interface) error {
	stop := node.Command("bash", "/opt/deckhouse/registry/bootstrap-seed/stop_registry_seed.sh")
	stop.Sudo(ctx)
	stop.WithTimeout(60 * time.Second)
	if _, _, err := stop.Output(ctx); err != nil {
		log.GetDefaultLogger().LogWarnLn("registry seed teardown: stop script:", err)
	}

	rm := node.Command("rm", "-rf",
		"/opt/deckhouse/registry/bootstrap-data",
		"/opt/deckhouse/registry/bootstrap-seed",
	)
	rm.Sudo(ctx)
	rm.WithTimeout(60 * time.Second)
	if _, _, err := rm.Output(ctx); err != nil {
		return fmt.Errorf("remove seed store: %w", err)
	}
	return nil
}

// cleanupSyncerConfig removes the temporary syncer config file from the master.
func cleanupSyncerConfig(ctx context.Context, node libcon.Interface) error {
	cmd := node.Command("rm", "-f", syncerConfigPath)
	cmd.Sudo(ctx)
	cmd.WithTimeout(10 * time.Second)
	_, _, err := cmd.Output(ctx)
	return err
}

// WaitForCacheAndAgentReady polls the Kubernetes API until both the registry-cache
// StatefulSet (≥1 ready replica, leader elected) and the registry-agent DaemonSet
// (all desired pods Ready) are healthy.
//
// Readiness predicate chosen:
//
//	registry-cache: StatefulSet readyReplicas == spec.replicas && replicas >= 1
//	registry-agent: DaemonSet numberReady == desiredNumberScheduled
//
// Queries go through the in-process kube client (the same one that installed
// Deckhouse), not node-side kubectl: dhctl already holds an API client here, and
// shelling kubectl over SSH+sudo added a PATH dependency (/opt/deckhouse/bin).
//
// TODO(e2e-gate): Exercised by the air-gap bootstrap e2e assert-statefulset-replicas step.
func WaitForCacheAndAgentReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for registry-cache + registry-agent to become Ready", cacheReadyPollAttempts, cacheReadyPollWait).
		RunContext(ctx, func() error {
			return checkCacheAndAgentReady(ctx, kubeCl)
		})
}

// checkCacheAndAgentReady performs a single readiness probe for the cache
// StatefulSet and agent DaemonSet via the kube API.
func checkCacheAndAgentReady(ctx context.Context, kubeCl *client.KubernetesClient) error {
	// Check registry-cache StatefulSet: readyReplicas == spec.replicas >= 1
	sts, err := kubeCl.AppsV1().StatefulSets("d8-system").Get(ctx, "registry-cache", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get statefulset registry-cache: %w", err)
	}
	var desired int32
	if sts.Spec.Replicas != nil {
		desired = *sts.Spec.Replicas
	}
	if desired < 1 || sts.Status.ReadyReplicas < desired {
		return fmt.Errorf("registry-cache not ready (desired=%d ready=%d)", desired, sts.Status.ReadyReplicas)
	}

	// Check registry-agent DaemonSet: numberReady == desiredNumberScheduled
	ds, err := kubeCl.AppsV1().DaemonSets("d8-system").Get(ctx, "registry-agent", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get daemonset registry-agent: %w", err)
	}
	if ds.Status.DesiredNumberScheduled < 1 || ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		return fmt.Errorf("registry-agent not ready (desired=%d ready=%d)", ds.Status.DesiredNumberScheduled, ds.Status.NumberReady)
	}

	return nil
}
