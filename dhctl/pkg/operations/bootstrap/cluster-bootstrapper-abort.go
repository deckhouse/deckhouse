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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

func (b *ClusterBootstrapper) Abort(ctx context.Context, forceAbortFromCache bool) error {
	restore := b.applyParams()
	defer restore()

	if !app.SanityCheck {
		log.WarnLn(bootstrapAbortCheckMessage)
	}

	return log.ProcessCtx(ctx, "bootstrap", "Abort", func(ctx context.Context) error {
		return b.doRunBootstrapAbort(ctx, forceAbortFromCache)
	})
}

func (b *ClusterBootstrapper) doRunBootstrapAbort(ctx context.Context, forceAbortFromCache bool) error {
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
		b.DirectoryConfig,
	)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(
		phases.OperationDestroy, b.Params.OnPhaseFunc, b.Params.OnProgressFunc,
	)

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           b.logger,
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, b.logger)

	cachePath := metaConfig.CachePath()
	log.InfoF("State config for prefix %s:  %s\n", metaConfig.ClusterPrefix, cachePath)
	if err = cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		return fmt.Errorf(bootstrapAbortInvalidCacheMessage, cachePath, err)
	}
	stateCache := cache.Global()

	if err := b.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	defer func() {
		_ = b.PhasedExecutionContext.Finalize(ctx, stateCache)
	}()

	hasUUID, err := stateCache.InCache(ctx, "uuid")
	if err != nil {
		return fmt.Errorf("unable to check uuid: %w", err)
	}

	if !hasUUID {
		if b.CommanderMode {
			log.InfoF("No UUID found in the cache, will exit now\n")
			return nil
		}
		return fmt.Errorf("No UUID found in the cache. Perhaps, the cluster was already bootstrapped.")
	}

	err = log.ProcessCtx(ctx, "common", "Get cluster UUID from the cache", func(ctx context.Context) error {
		uuid, err := stateCache.Load(ctx, "uuid")
		if err != nil {
			return err
		}
		metaConfig.UUID = string(uuid)
		log.InfoF("Cluster UUID: %s\n", metaConfig.UUID)
		return nil
	})
	if err != nil {
		return err
	}

	// init ssh client is safe if master hosts not found (error in base infra)

	var destroyer destroy.Destroyer

	loggerProvider := log.SimpleLoggerProvider(b.Logger)

	bootstrapState := NewBootstrapState(stateCache)

	err = log.ProcessCtx(ctx, "common", "Choice abort type", func(ctx context.Context) error {
		ok, err := bootstrapState.IsManifestsCreated(ctx)
		if err != nil {
			return err
		}

		b.KubeProvider = b.SSHProviderInitializer.GetKubeProvider(ctx)
		// error is OK here in case of abort from cache w/o ssh hosts
		sshProvider, _ := b.SSHProviderInitializer.GetSSHProvider(ctx)

		log.DebugF("Abort from cache. tf-state-and-manifests-in-cluster=%v; Force abort %v\n", ok, forceAbortFromCache)
		if !ok || forceAbortFromCache {
			destroyer, err = destroy.GetAbortDestroyer(ctx, &destroy.GetAbortDestroyerParams{
				MetaConfig:             metaConfig,
				StateCache:             stateCache,
				InfrastructureContext:  b.InfrastructureContext,
				PhasedExecutionContext: b.PhasedExecutionContext,

				SSHClientProvider: sshProvider,
				LoggerProvider:    loggerProvider,

				TmpDir:        b.TmpDir,
				IsDebug:       b.IsDebug,
				CommanderMode: b.CommanderMode,
			})
			if err != nil {
				return err
			}

			logMsg := "Deckhouse installation was not started before. Abort from cache"
			if forceAbortFromCache {
				logMsg = "Force aborting from cache"
			}

			log.InfoLn(logMsg)

			return nil
		}

		destroyParams := &destroy.Params{
			StateCache:             cache.Global(),
			PhasedExecutionContext: b.PhasedExecutionContext,
			SkipResources:          app.SkipResources,
			InfrastructureContext:  b.InfrastructureContext,
			DirectoryConfig:        b.DirectoryConfig,
			SSHProvider:            sshProvider,
			KubeProvider:           b.KubeProvider,
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

		destroyParams.LoggerProvider = loggerProvider
		destroyParams.IsDebug = b.IsDebug
		destroyParams.TmpDir = b.TmpDir

		destroyer, err = destroy.NewClusterDestroyer(ctx, destroyParams)
		if err != nil {
			return err
		}

		log.InfoLn("Deckhouse installation was started before. Destroy cluster")
		return nil
	})
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.SetClusterConfig(phases.ClusterConfig{ClusterType: metaConfig.ClusterType})

	if metaConfig.IsStatic() {
		deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
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
		preflightRunner.SetCacheSalt(state.ConfigHash(app.ConfigPaths))
		preflightRunner.DisableChecks(app.DisabledPreflightChecks()...)
		if err := preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}

	}

	if govalue.IsNil(destroyer) {
		return fmt.Errorf("Destroyer not initialized")
	}

	// destroy cluster cleanup provider
	if err := destroyer.DestroyCluster(ctx, app.SanityCheck); err != nil {
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
