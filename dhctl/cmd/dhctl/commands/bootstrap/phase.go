// Copyright 2021 Flant JSC
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

	"gopkg.in/alecthomas/kingpin.v2"

	destroycmd "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	terrastate "github.com/deckhouse/deckhouse/dhctl/pkg/state/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/ssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terminal"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/tomb"
)

func DefineBootstrapInstallDeckhouseCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install-deckhouse", "Install deckhouse and wait for its readiness.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDeckhouseInstallFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		installConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		installConfig.KubeadmBootstrap = app.KubeadmBootstrap
		installConfig.MasterNodeSelector = app.MasterNodeSelector

		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
		if err != nil {
			return err
		}

		return operations.InstallDeckhouse(kubeCl, installConfig)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("execute-bashible-bundle", "Prepare Master node and install Kubernetes.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineBashibleBundleFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		sshClient, err := ssh.NewClientFromFlagsWithHosts()
		if err != nil {
			return err
		}

		sshClient, err = sshClient.Start()
		if err != nil {
			return err
		}

		err = terminal.AskBecomePassword()
		if err != nil {
			return err
		}

		if err := operations.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}

		return operations.RunBashiblePipeline(sshClient, metaConfig, app.InternalNodeIP, app.DevicePath)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}

func DefineCreateResourcesCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-resources", "Create resources in Kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineResourcesFlags(cmd, true)
	app.DefineKubeFlags(cmd)

	runFunc := func() error {
		var resourcesToCreate template.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := template.ParseResources(app.ResourcesPath, nil)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		if len(resourcesToCreate) == 0 {
			log.WarnLn("Resources to create were not found.")
			return nil
		}

		sshClient, err := ssh.NewInitClientFromFlags(true)
		if err != nil {
			return err
		}

		return log.Process("bootstrap", "Create resources", func() error {
			kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
			if err != nil {
				return err
			}

			checkers, err := resources.GetCheckers(kubeCl, resourcesToCreate, nil)
			if err != nil {
				return err
			}

			return resources.CreateResourcesLoop(kubeCl, resourcesToCreate, checkers)
		})
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}

const (
	bootstrapAbortInvalidCacheMessage = `Create cache %s:
	Error: %v
	Probably that Kubernetes cluster was successfully bootstrapped.
	Use "dhctl destroy" command to delete the cluster.
`
	bootstrapAbortCheckMessage = `You will be asked for approval multiple times.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
)

type Destroyer interface {
	DestroyCluster(autoApprove bool) error
}

func DefineBootstrapAbortCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("abort", "Delete every node, which was created during bootstrap process.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)
	app.DefineAbortFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		cachePath := metaConfig.CachePath()
		log.InfoF("State config for prefix %s:  %s", metaConfig.ClusterPrefix, cachePath)
		if err = cache.Init(cachePath); err != nil {
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

		var destroyer Destroyer

		err = log.Process("common", "Choice abort type", func() error {
			ok, err := stateCache.InCache(operations.ManifestCreatedInClusterCacheKey)
			if err != nil {
				return err
			}
			if !ok || app.ForceAbortFromCache {
				log.DebugF(fmt.Sprintf("Abort from cache. tf-state-and-manifests-in-cluster=%v; Force abort %v\n", ok, app.ForceAbortFromCache))
				terraStateLoader := terrastate.NewFileTerraStateLoader(stateCache, metaConfig)
				destroyer = infrastructure.NewClusterInfra(terraStateLoader, stateCache)

				logMsg := "Deckhouse installation was not started before. Abort from cache"
				if app.ForceAbortFromCache {
					logMsg = "Force aborting from cache"
				}

				log.InfoLn(logMsg)

				return nil
			}

			mastersIPs, err := operations.GetMasterHostsIPs()
			if err != nil {
				return err
			}
			app.SSHHosts = mastersIPs

			bastionHost, err := operations.GetBastionHostFromCache()
			if err != nil {
				log.ErrorF("Can not load bastion host: %v\n", err)
				return err
			}

			if bastionHost != "" {
				setBastionHostFromCloudProvider(bastionHost, nil)
			}

			destroyer, err = destroycmd.InitClusterDestroyer()
			if err != nil {
				return err
			}

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

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.WarnLn(bootstrapAbortCheckMessage)
		}

		return log.Process("bootstrap", "Abort", func() error { return runFunc() })
	})

	return cmd
}

const bootstrapPhaseBaseInfraNonCloudMessage = `It is impossible to create base-infrastructure for non-cloud Kubernetes cluster.
You have to create it manually.
`

func DefineBaseInfrastructureCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("base-infra", "Create base infrastructure for Cloud Kubernetes cluster.")
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		if metaConfig.ClusterType != config.CloudClusterType {
			return fmt.Errorf(bootstrapPhaseBaseInfraNonCloudMessage)
		}

		cachePath := metaConfig.CachePath()
		if err = cache.Init(cachePath); err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(cacheMessage, cachePath, err)
		}

		stateCache := cache.Global()

		if app.DropCache {
			stateCache.Clean()
			stateCache.Delete(state.TombstoneKey)
		}

		clusterUUID, err := generateClusterUUID(stateCache)
		if err != nil {
			return err
		}
		metaConfig.UUID = clusterUUID

		return log.Process("bootstrap", "Cloud infrastructure", func() error {
			baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure", stateCache).
				WithVariables(metaConfig.MarshalConfig()).
				WithAutoApprove(true)
			tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

			_, err := terraform.ApplyPipeline(baseRunner, "Kubernetes cluster", terraform.GetBaseInfraResult)
			return err
		})
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return runFunc()
	})

	return cmd
}

func DefineExecPostBootstrapScript(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("exec-post-bootstrap", "Test scp upload and ssh run uploaded script.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewInitClientFromFlagsWithHosts(true)
		if err != nil {
			return nil
		}

		if err = cache.Init(sshClient.Check().String()); err != nil {
			return fmt.Errorf("Can not init cache: %v", err)
		}

		bootstrapState := bootstrap.NewBootstrapState(cache.Global())

		postScriptExecutor := bootstrap.NewPostBootstrapScriptExecutor(sshClient, app.PostBootstrapScriptPath, bootstrapState).
			WithTimeout(app.PostBootstrapScriptTimeout)

		if err := postScriptExecutor.Execute(); err != nil {
			return err
		}

		out, err := bootstrapState.PostBootstrapScriptResult()
		if err != nil {
			return err
		}

		fmt.Printf("Output from post-bootstrap script:\n%s", string(out))

		return nil
	})

	return cmd
}
