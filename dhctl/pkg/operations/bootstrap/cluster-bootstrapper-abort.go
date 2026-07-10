// Copyright 2023 Flant JSC
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

package bootstrap

import (
	"context"
	"fmt"

	"github.com/name212/govalue"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

func (b *ClusterBootstrapper) Abort(ctx context.Context, forceAbortFromCache bool) error {
	if !b.Options.Global.SanityCheck {
		dhlog.FromContext(ctx).WarnContext(ctx, bootstrapAbortCheckMessage)
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Abort", func(ctx context.Context) error {
		return b.doRunBootstrapAbort(ctx, forceAbortFromCache)
	})
}

func (b *ClusterBootstrapper) doRunBootstrapAbort(ctx context.Context, forceAbortFromCache bool) error {
	// Registry shoud run before LoadConfigFromFile
	registryStop, err := registry.InitFromConfig(
		ctx,
		dhlog.FromContext(ctx),
		b.Options.Global.ConfigPaths,
		b.Options.Registry.ImgBundlePath,
	)
	if err != nil {
		return err
	}
	defer registryStop()

	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(),
		),
		&b.Options.Global,
	)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(
		phases.OperationDestroy, b.Params.OnPhaseFunc, b.Params.OnProgressFunc,
	)

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		GlobalOptions:    &b.Options.Global,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter).
		WithUseTfCache(b.Options.Cache.UseTfCache).
		WithDebug(b.Options.Global.IsDebug)

	cachePath := metaConfig.CachePath()
	dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("State config for prefix %s:  %s", metaConfig.ClusterPrefix, cachePath))
	if err = cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState, Cache: b.Options.Cache}); err != nil {
		return fmt.Errorf(bootstrapAbortInvalidCacheMessage, cachePath, err)
	}
	stateCache := cache.Global()

	if err := b.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	defer func() {
		if err := b.PhasedExecutionContext.Finalize(ctx, stateCache); err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to finalize phased execution context: %v", err))
		}
	}()

	hasUUID, err := stateCache.InCache(ctx, "uuid")
	if err != nil {
		return fmt.Errorf("unable to check uuid: %w", err)
	}

	if !hasUUID {
		return b.commanderModeAction(
			func() error {
				dhlog.FromContext(ctx).InfoContext(ctx, "No UUID found in the cache, will exit now")
				return nil
			},
			func() error {
				return fmt.Errorf("No UUID found in the cache. Perhaps the cluster was already bootstrapped.")
			},
		)
	}

	err = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Get cluster UUID from the cache", func(ctx context.Context) error {
		uuid, err := stateCache.Load(ctx, "uuid")
		if err != nil {
			return err
		}
		metaConfig.UUID = string(uuid)
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Cluster UUID: %s", metaConfig.UUID))
		return nil
	})
	if err != nil {
		return err
	}

	// init ssh client is safe if master hosts not found (error in base infra)

	var destroyer destroy.Destroyer

	bootstrapState := NewBootstrapState(stateCache)

	err = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Choose abort type", func(ctx context.Context) error {
		ok, err := bootstrapState.IsManifestsCreated(ctx)
		if err != nil {
			return err
		}

		b.KubeProvider = b.SSHProviderInitializer.GetKubeProvider(ctx)
		// error is OK here in case of abort from cache w/o ssh hosts
		sshProvider, _ := b.SSHProviderInitializer.GetSSHProvider(ctx)

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Abort from cache. tf-state-and-manifests-in-cluster=%v; Force abort %v", ok, forceAbortFromCache))
		if !ok || forceAbortFromCache {
			destroyer, err = destroy.GetAbortDestroyer(ctx, &destroy.GetAbortDestroyerParams{
				MetaConfig:             metaConfig,
				StateCache:             stateCache,
				InfrastructureContext:  b.InfrastructureContext,
				PhasedExecutionContext: b.PhasedExecutionContext,

				SSHClientProvider: sshProvider,
				Logger:            dhlog.FromContext(ctx),

				TmpDir:        b.TmpDir,
				GlobalOptions: &b.Options.Global,
				IsDebug:       b.IsDebug,
				CommanderMode: b.CommanderMode,
				SSHUser:       b.Options.SSH.User,
			})
			if err != nil {
				return err
			}

			logMsg := "Deckhouse installation has not started yet. Aborting from cache"
			if forceAbortFromCache {
				logMsg = "Force aborting from cache"
			}

			dhlog.FromContext(ctx).InfoContext(ctx, logMsg)

			return nil
		}

		destroyParams := &destroy.Params{
			StateCache:             cache.Global(),
			PhasedExecutionContext: b.PhasedExecutionContext,
			SkipResources:          b.Options.Destroy.SkipResources,
			InfrastructureContext:  b.InfrastructureContext,
			SSHProvider:            sshProvider,
			KubeProvider:           b.KubeProvider,
			Options:                b.Options,
		}

		if b.CommanderMode {
			clusterConfigurationData, err := metaConfig.ClusterConfigYAML()
			if err != nil {
				return err
			}
			providerClusterConfigurationData, err := metaConfig.ProviderClusterConfigYAML()
			if err != nil {
				return err
			}
			destroyParams.CommanderMode = true
			destroyParams.CommanderModeParams = commander.NewCommanderModeParams(clusterConfigurationData, providerClusterConfigurationData)
		}

		destroyParams.Logger = dhlog.FromContext(ctx)
		destroyParams.IsDebug = b.IsDebug
		destroyParams.TmpDir = b.TmpDir

		destroyer, err = destroy.NewClusterDestroyer(ctx, destroyParams)
		if err != nil {
			return err
		}

		dhlog.FromContext(ctx).InfoContext(ctx, "Deckhouse installation has already started. Destroying cluster")
		return nil
	})
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	if metaConfig.IsStatic() {
		deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig, &b.Options.Global)
		if err != nil {
			return err
		}

		if b.CommanderMode {
			deckhouseInstallConfig.CommanderMode = b.CommanderMode
			deckhouseInstallConfig.CommanderUUID = b.CommanderUUID
		}

		staticAbortSuite, err := suites.NewStaticAbortSuite(suites.StaticAbortDeps{SSHProviderInitializer: b.SSHProviderInitializer}, ctx)
		if err != nil {
			return err
		}
		preflightRunner := preflight.New(staticAbortSuite)
		preflightRunner.UseCache(bootstrapState)
		preflightRunner.SetCacheSalt(state.ConfigHash(ctx, b.Options.Global.ConfigPaths))
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
		if err := preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}
	}

	if govalue.IsNil(destroyer) {
		return fmt.Errorf("Destroyer not initialized")
	}

	// destroy cluster cleanup provider
	if err := destroyer.DestroyCluster(ctx, b.Options.Global.SanityCheck); err != nil {
		b.lastState = b.PhasedExecutionContext.GetLastState()
		return err
	}
	if err := b.PhasedExecutionContext.CompletePipeline(ctx, stateCache); err != nil {
		b.lastState = b.PhasedExecutionContext.GetLastState()
		return err
	}
	b.lastState = b.PhasedExecutionContext.GetLastState()

	stateCache.Clean(ctx)
	// Allow to reuse cache because cluster will be bootstrapped again (probably)
	stateCache.Delete(ctx, state.TombstoneKey)

	return nil
}
