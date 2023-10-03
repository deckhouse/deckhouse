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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	terrastate "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
)

func (b *ClusterBootstrapper) Abort(forceAbortFromCache bool) error {
	if restore, err := b.applyParams(); err != nil {
		return err
	} else {
		defer restore()
	}

	if !app.SanityCheck {
		log.WarnLn(bootstrapAbortCheckMessage)
	}

	return log.Process("bootstrap", "Abort", func() error { return b.doRunBootstrapAbort(forceAbortFromCache) })
}

func getSSHClient() (*ssh.Client, error) {
	mastersIPs, err := GetMasterHostsIPs()
	if err != nil {
		return nil, err
	}
	app.SSHHosts = mastersIPs

	bastionHost, err := GetBastionHostFromCache()
	if err != nil {
		log.ErrorF("Can not load bastion host: %v\n", err)
		return nil, err
	}

	if bastionHost != "" {
		setBastionHost(bastionHost, nil)
	}

	return ssh.NewClientFromFlags().Start()
}

func (b *ClusterBootstrapper) doRunBootstrapAbort(forceAbortFromCache bool) error {
	metaConfig, err := config.ParseConfig(app.ConfigPath)
	if err != nil {
		return err
	}

	cachePath := metaConfig.CachePath()
	log.InfoF("State config for prefix %s:  %s", metaConfig.ClusterPrefix, cachePath)
	if err = cache.InitWithOptions(cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState}); err != nil {
		return fmt.Errorf(bootstrapAbortInvalidCacheMessage, cachePath, err)
	}
	stateCache := cache.Global()

	hasUUID, err := stateCache.InCache("uuid")
	if err != nil {
		return err
	}

	if !hasUUID {
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
				terraStateLoader := terrastate.NewFileTerraStateLoader(stateCache, metaConfig)
				destroyer = infrastructure.NewClusterInfra(terraStateLoader, stateCache)
			} else {
				sshClient, err := getSSHClient()
				if err != nil {
					return err
				}
				destroyer = destroy.NewStaticMastersDestroyer(sshClient)
			}

			logMsg := "Deckhouse installation was not started before. Abort from cache"
			if forceAbortFromCache {
				logMsg = "Force aborting from cache"
			}

			log.InfoLn(logMsg)

			return nil
		}

		sshClient, err := getSSHClient()
		if err != nil {
			return err
		}

		if err := terminal.AskBecomePassword(); err != nil {
			return err
		}
		if err = cache.InitWithOptions(sshClient.Check().String(), cache.CacheOptions{}); err != nil {
			return fmt.Errorf(bootstrapAbortInvalidCacheMessage, sshClient.Check().String(), err)
		}
		destroyer = destroy.NewClusterDestroyer(&destroy.Params{
			SSHClient:   sshClient,
			StateCache:  cache.Global(),
			OnPhaseFunc: b.OnPhaseFunc,

			SkipResources: app.SkipResources,
		})

		log.InfoLn("Deckhouse installation was started before. Destroy cluster")
		return nil
	})

	if err != nil {
		return err
	}

	if destroyer == nil {
		return fmt.Errorf("Destroyer not initialized")
	}

	if err := destroyer.DestroyCluster(app.SanityCheck); err != nil {
		return err
	}

	stateCache.Clean()
	// Allow to reuse cache because cluster will be bootstrapped again (probably)
	stateCache.Delete(state.TombstoneKey)

	return nil
}
