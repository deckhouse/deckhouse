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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type InstallDeckhouseResult struct {
	ManifestResult *deckhouse.ManifestsResult
}

type InstallDeckhouseParams struct {
	BeforeDeckhouseTask func() error
	DeckhouseTimeout    time.Duration
}

func InstallDeckhouse(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	config *config.DeckhouseInstaller,
	params InstallDeckhouseParams,
) (*InstallDeckhouseResult, error) {
	res := &InstallDeckhouseResult{}

	return res, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Install Deckhouse", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "InstallDeckhouse")
		defer span.End()

		if err := resolveClusterUUID(ctx, kubeCl, config); err != nil {
			return err
		}

		err := CheckPreventBreakAnotherBootstrappedCluster(ctx, kubeCl, config)
		if err != nil {
			return err
		}

		// Install the ModuleConfig CRD before pre-Deckhouse resources and
		// ModuleConfig manifests are applied, so they don't have to wait for
		// deckhouse-controller to start. It is a file-based precondition (with
		// version-merge semantics matching deckhouse-controller's EnsureCRDs),
		// not a single-object manifest, so it lives here rather than inside the
		// CreateDeckhouseManifests task list. No-op (with a warning) when the
		// CRD file is unavailable.
		if err := deckhouse.EnsureModuleConfigCRD(ctx, kubeCl, config.ModuleConfigCRDPath); err != nil {
			return fmt.Errorf("ensure ModuleConfig CRD: %w", err)
		}

		resManifests, err := deckhouse.CreateDeckhouseManifests(ctx, kubeCl, config, params.BeforeDeckhouseTask)
		if err != nil {
			return fmt.Errorf("create Deckhouse manifests: %w", err)
		}

		res.ManifestResult = resManifests

		err = deckhouse.WaitForReadiness(ctx, kubeCl, params.DeckhouseTimeout)
		if err != nil {
			return fmt.Errorf("Deckhouse not ready: %w", err)
		}

		err = registry_config.WaitForRegistryReady(ctx, kubeCl, config.Registry)
		if err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})
}

// resolveClusterUUID decides which UUID this installation writes to kube-system/d8-cluster-uuid,
// and it is here rather than in either caller because both reach the cluster for the first time
// through this function - the standalone install-deckhouse phase and the InstallDeckhouse node of
// a full bootstrap.
//
// dhctl may keep a UUID of its own only for a cluster it created, which is what a
// ClusterConfiguration means here. For such a cluster dhctl's value wins: taking the cluster's
// instead would disarm CheckPreventBreakAnotherBootstrappedCluster below. Every other cluster is
// the sole authority on its own identity, so its stamp wins, and an unstamped one is given a fresh
// UUID rather than whatever the run arrived carrying - install.go writes into the cluster whatever
// survives here, and a value from a state cache belongs to whoever wrote it there.
func resolveClusterUUID(ctx context.Context, kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
	if cfg.UUID != "" && len(cfg.ClusterConfig) > 0 {
		return nil
	}

	// This is the first API call of both callers, made against an API server that may still be
	// starting, and it is retried exactly as long as the read of the same object in
	// CheckPreventBreakAnotherBootstrappedCluster right after it. Absent is an answer rather than a
	// failure, so an unstamped cluster reaches the generate branch on the first attempt; every
	// other error must exhaust the loop instead, because falling through to a fresh UUID would
	// stamp a second identity into a cluster that already has one.
	loopParams := retry.NewEmptyParams(
		retry.WithName("Read cluster UUID"),
		retry.WithAttempts(45),
		retry.WithWait(1*time.Second),
		retry.WithWhitelist(ErrClusterUUIDCheckTransient),
	)

	var uuidInCluster string

	err := retry.NewSilentLoopWithParams(loopParams).RunContext(ctx, func() error {
		cm, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Get(ctx, manifests.ClusterUUIDCm, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w: %w", ErrClusterUUIDCheckTransient, err)
		}

		uuidInCluster = cm.Data[manifests.ClusterUUIDCmKey]

		return nil
	})
	if err != nil {
		return fmt.Errorf("read cluster UUID config map: %w", err)
	}

	if uuidInCluster != "" {
		cfg.UUID = uuidInCluster
		return nil
	}

	cfg.UUID = uuid.New().String()

	return nil
}

func applyPostBootstrapModuleConfigs(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	tasks []actions.ModuleConfigTask,
) error {
	ctx, span := telemetry.StartSpan(ctx, "applyPostBootstrapModuleConfigs") //nolint:ineffassign,staticcheck // ctx reassigned for span propagation to future calls
	defer span.End()

	for _, task := range tasks {
		p := retry.NewEmptyParams(
			retry.WithName("%s", task.Title),
			retry.WithAttempts(75),
			retry.WithWait(1*time.Second),
			retry.WithLogger(dhlog.FromContext(ctx)),
			retry.WithWhitelist(actions.ErrManifestTaskTransient),
		)
		err := retry.NewLoopWithParams(p).
			Run(func() error {
				return task.Do(kubeCl)
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func RunPostInstallTasks(ctx context.Context, kubeCl *client.KubernetesClient, result *InstallDeckhouseResult) error {
	ctx, span := telemetry.StartSpan(ctx, "RunPostInstallTasks")
	defer span.End()

	if result == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "Skipping post-install tasks because result is nil")
		return nil
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Run post bootstrap actions", func(ctx context.Context) error {
		return applyPostBootstrapModuleConfigs(ctx, kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
