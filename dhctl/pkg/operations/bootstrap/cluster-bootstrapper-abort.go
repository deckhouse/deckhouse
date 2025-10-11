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
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure/controller"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	infrastructurestate "github.com/deckhouse/deckhouse/dhctl/pkg/state/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func (b *ClusterBootstrapper) Abort(ctx context.Context, forceAbortFromCache bool) error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	if !app.SanityCheck {
		log.WarnLn(bootstrapAbortCheckMessage)
	}

	return log.Process("bootstrap", "Abort", func() error { return b.doRunBootstrapAbort(ctx, forceAbortFromCache) })
}

func (b *ClusterBootstrapper) initSSHClient() error {
	wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper)
	if !ok {
		return nil // Local runs don't use ssh client.
	}

	if err := terminal.AskBecomePassword(); err != nil {
		return err
	}
	if err := terminal.AskBastionPassword(); err != nil {
		return err
	}

	sshClient := wrapper.Client()

	if len(sshClient.Session().AvailableHosts()) == 0 {
		mastersIPs, err := GetMasterHostsIPs()
		if err != nil {
			log.ErrorF("Can not load available ssh hosts: %v\n", err)
			return err
		}
		sshClient.Session().SetAvailableHosts(mastersIPs)
	}

	bastionHost, err := GetBastionHostFromCache()
	if err != nil {
		log.ErrorF("Can not load bastion host: %v\n", err)
		return fmt.Errorf("unable to load bastion host: %w", err)
	}

	if bastionHost != "" {
		sshClient.Session().BastionHost = bastionHost
	}

	if err := sshClient.Start(); err != nil {
		return fmt.Errorf("unable to start ssh client: %w", err)
	}

	return nil
}

func (b *ClusterBootstrapper) doRunBootstrapAbort(ctx context.Context, forceAbortFromCache bool) error {
	metaConfig, err := config.ParseConfig(
		ctx,
		app.ConfigPaths,
		infrastructureprovider.MetaConfigPreparatorProvider(
			infrastructureprovider.NewPreparatorProviderParams(b.logger),
		),
	)
	if err != nil {
		return err
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		Logger:           b.logger,
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter, b.logger)

	cachePath := metaConfig.CachePath()
	log.InfoF("State config for prefix %s:  %s\n", metaConfig.ClusterPrefix, cachePath)
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		return fmt.Errorf(bootstrapAbortInvalidCacheMessage, cachePath, err)
	}
	stateCache := cache.Global()

	hasUUID, err := stateCache.InCache("uuid")
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

	err = log.Process("common", "Get cluster UUID from the cache", func() error {
		uuid, err := stateCache.Load("uuid")
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

	var destroyer destroy.Destroyer

	err = log.Process("common", "Choice abort type", func() error {
		ok, err := stateCache.InCache(ManifestCreatedInClusterCacheKey)
		if err != nil {
			return err
		}
		if !ok || forceAbortFromCache {
			log.DebugF(fmt.Sprintf("Abort from cache. tf-state-and-manifests-in-cluster=%v; Force abort %v\n", ok, forceAbortFromCache))
			if metaConfig.ClusterType == config.CloudClusterType {
				terraStateLoader := infrastructurestate.NewFileTerraStateLoader(stateCache, metaConfig)
				destroyer = controller.NewClusterInfraWithOptions(
					terraStateLoader, stateCache, b.InfrastructureContext,
					controller.ClusterInfraOptions{
						PhasedExecutionContext: b.PhasedExecutionContext,
						TmpDir:                 b.TmpDir,
						Logger:                 b.Logger,
						IsDebug:                b.IsDebug,
					},
				)
			} else {
				wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper)
				if !ok {
					return fmt.Errorf("destroy operations are not supported for local execution contexts")
				}
				if err := b.initSSHClient(); err != nil {
					return err
				}

				sshClientProvider := func() (node.SSHClient, error) {
					// client initialized above
					return wrapper.Client(), nil
				}

				destroyer = destroy.NewStaticMastersDestroyer(sshClientProvider, []destroy.NodeIP{})
			}

			logMsg := "Deckhouse installation was not started before. Abort from cache"
			if forceAbortFromCache {
				logMsg = "Force aborting from cache"
			}

			log.InfoLn(logMsg)

			return nil
		}

		if err := b.initSSHClient(); err != nil {
			return err
		}

		if !b.CommanderMode {
			if wrapper, ok := b.NodeInterface.(*ssh.NodeInterfaceWrapper); ok {
				if err = cache.InitWithOptions(wrapper.Client().Check().String(), cache.CacheOptions{}); err != nil {
					return fmt.Errorf(bootstrapAbortInvalidCacheMessage, wrapper.Client().Check().String(), err)
				}
			}
		}

		destroyParams := &destroy.Params{
			NodeInterface:          b.NodeInterface,
			StateCache:             cache.Global(),
			PhasedExecutionContext: b.PhasedExecutionContext,
			SkipResources:          app.SkipResources,
			InfrastructureContext:  b.InfrastructureContext,
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

		destroyParams.Logger = b.logger
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

	if metaConfig.IsStatic() {
		deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		if b.CommanderMode {
			deckhouseInstallConfig.CommanderMode = b.CommanderMode
			deckhouseInstallConfig.CommanderUUID = b.CommanderUUID
		}
		bootstrapState := NewBootstrapState(stateCache)
		preflightChecker := preflight.NewChecker(b.NodeInterface, deckhouseInstallConfig, metaConfig, bootstrapState)
		if err := preflightChecker.StaticSudo(ctx); err != nil {
			return err
		}
	}

	if govalue.IsNil(destroyer) {
		return fmt.Errorf("Destroyer not initialized")
	}

	if err := b.PhasedExecutionContext.InitPipeline(stateCache); err != nil {
		return err
	}
	defer b.PhasedExecutionContext.Finalize(stateCache)

	// destroy cluster cleanup provider
	if err := destroyer.DestroyCluster(ctx, app.SanityCheck); err != nil {
		b.lastState = b.PhasedExecutionContext.GetLastState()
		return err
	}
	if err := b.PhasedExecutionContext.CompletePipeline(stateCache); err != nil {
		b.lastState = b.PhasedExecutionContext.GetLastState()
		return err
	}
	b.lastState = b.PhasedExecutionContext.GetLastState()

	stateCache.Clean()
	// Allow to reuse cache because cluster will be bootstrapped again (probably)
	stateCache.Delete(state.TombstoneKey)

	return nil
}
