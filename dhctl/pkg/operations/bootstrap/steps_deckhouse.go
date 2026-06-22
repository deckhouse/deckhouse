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

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	registry_ops "github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type InstallDeckhouseResult struct {
	ManifestResult *deckhouse.ManifestsResult
}

type InstallDeckhouseParams struct {
	BeforeDeckhouseTask func() error
	State               *State
	DeckhouseTimeout    time.Duration
	// Node is the first-master connection used by the air-gap (NeedsSeed) registry
	// finalize (cache-ready wait, seed->cache fill, seed teardown). It is required
	// only when config.Registry.NeedsSeed() is true (air-gap requires SSH); for
	// non-seed installs and resume/non-SSH callers it may be nil.
	Node libcon.Interface
}

func InstallDeckhouse(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	config *config.DeckhouseInstaller,
	params InstallDeckhouseParams,
) (*InstallDeckhouseResult, error) {
	res := &InstallDeckhouseResult{}

	return res, log.ProcessCtx(ctx, "bootstrap", "Install Deckhouse", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "InstallDeckhouse")
		defer span.End()

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

		if err := params.State.SaveManifestsCreated(ctx); err != nil {
			return fmt.Errorf("set the manifests-in-cluster flag in the cache: %w", err)
		}

		err = deckhouse.WaitForReadiness(ctx, kubeCl, params.DeckhouseTimeout)
		if err != nil {
			return fmt.Errorf("Deckhouse not ready: %w", err)
		}

		// Registry new-arch finalize. Air-gap (NeedsSeed) only: the cache must come
		// up and be filled from the on-node seed before the seed is torn down.
		// Direct/Proxy need none of this (no seed, cache pull-through or off).
		if config.Registry.NeedsSeed() {
			// air-gap (NeedsSeed) requires an SSH connection (the bootstrap tunnel).
			// The node interface is mandatory here; a nil Node means a caller wired
			// the finalize wrong, so fail clearly rather than nil-panic later.
			if params.Node == nil {
				return fmt.Errorf("registry finalize: air-gap (NeedsSeed) install requires a node connection, but none was provided")
			}

			if err := registry_ops.WaitForCacheAndAgentReady(ctx, params.Node); err != nil {
				return fmt.Errorf("registry cache/agent not ready: %w", err)
			}

			fillParams, err := registry_ops.CacheFillParamsFromInitSecret(ctx, params.Node)
			if err != nil {
				return fmt.Errorf("read registry-init for cache fill: %w", err)
			}
			if err := registry_ops.FillCacheFromSeed(ctx, fillParams); err != nil {
				return fmt.Errorf("fill cache from seed: %w", err)
			}
			if err := registry_ops.VerifyCacheNonEmpty(ctx, fillParams); err != nil {
				return fmt.Errorf("verify cache non-empty: %w", err)
			}
			if err := registry_ops.DeleteBootstrapSecret(ctx, params.Node); err != nil {
				return fmt.Errorf("delete registry-bootstrap secret: %w", err)
			}
			if err := registry_ops.TeardownSeed(ctx, params.Node); err != nil {
				return fmt.Errorf("teardown registry seed: %w", err)
			}
		}

		// Warning! This function must be called at the end of the Deckhouse installation phase.
		// At the end of this function, the registry-init secret is deleted,
		// which is used during DeckhouseInstall for certain registry operation modes.
		err = registry_config.WaitForRegistryInitialization(ctx, kubeCl, config.Registry)
		if err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})
}

func applyPostBootstrapModuleConfigs(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	tasks []actions.ModuleConfigTask,
) error {
	ctx, span := telemetry.StartSpan(ctx, "applyPostBootstrapModuleConfigs") //nolint:ineffassign,staticcheck // ctx reassigned for span propagation to future calls
	defer span.End()

	for _, task := range tasks {
		extLogger := log.ExternalLoggerProvider(log.GetDefaultLogger())
		p := retry.NewEmptyParams(
			retry.WithName("%s", task.Title),
			retry.WithAttempts(75),
			retry.WithWait(1*time.Second),
			retry.WithLogger(extLogger()),
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
		log.DebugF("Skipping post-install tasks because result is nil\n")
		return nil
	}

	return log.ProcessCtx(ctx, "bootstrap", "Run post bootstrap actions", func(ctx context.Context) error {
		return applyPostBootstrapModuleConfigs(ctx, kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
