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

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

// Abort rolls a half-finished bootstrap back from the state cache: it destroys the infrastructure
// dhctl itself created and cleans the hosts it touched, and it does that without the Kube API and
// without deleting anything inside the cluster. Deleting a cluster that is already running is the
// job of "dhctl destroy", which connects to it and removes its resources first.
func (b *ClusterBootstrapper) Abort(ctx context.Context) error {
	if !b.Options.Global.SanityCheck {
		dhlog.FromContext(ctx).WarnContext(ctx, bootstrapAbortCheckMessage)
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Abort", func(ctx context.Context) error {
		return b.doRunBootstrapAbort(ctx)
	})
}

func (b *ClusterBootstrapper) doRunBootstrapAbort(ctx context.Context) error {
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
		infrastructureprovider.MetaConfigValidatorProvider(),
		&b.Options.Global,
	)
	if err != nil {
		return err
	}

	// Refused here rather than by the loader, which serves bootstrap on a cluster whose control
	// plane dhctl did not create too. Without this the run would reach the cluster type only in
	// GetAbortDestroyer, after CachePath has already collapsed to the prefix-less directory every
	// such cluster shares and the pipeline has written its state into it.
	if !metaConfig.HasClusterConfiguration() {
		return fmt.Errorf("dhctl bootstrap-phase abort requires ClusterConfiguration: it destroys the infrastructure dhctl created from it, and finds the state of that infrastructure in a cache named after the cluster prefix and provider it carries")
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

	cachePath := cacheIdentity(ctx, metaConfig, connectionHosts(b.SSHProviderInitializer), b.Options.Kube, b.Options.Cache.Dir)
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
				if metaConfig.IsStatic() {
					return fmt.Errorf("No UUID found in the cache. Perhaps the cluster was already bootstrapped, or this abort was given different addresses than the bootstrap: a static cluster names its state cache after its --ssh-host list (or the SSHHost resources of --connection-config), so the abort has to repeat them.")
				}

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
	// error is OK here in case of abort from cache w/o ssh hosts
	sshProvider, _ := b.SSHProviderInitializer.GetSSHProvider(ctx)

	b.PhasedExecutionContext.SetClusterConfig(phaseClusterConfig(metaConfig))

	destroyer, err := destroy.GetAbortDestroyer(ctx, &destroy.GetAbortDestroyerParams{
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
		preflightRunner.UseCache(NewBootstrapState(stateCache))
		preflightRunner.SetCacheSalt(state.ConfigHash(ctx, b.Options.Global.ConfigPaths))
		preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
		if err := preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
			return err
		}
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
