package bootstrap

import (
	"fmt"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/actions/resources"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/terraform"
	"flant/candictl/pkg/util/cache"
	"flant/candictl/pkg/util/tomb"
)

func DefineBootstrapInstallDeckhouseCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install-deckhouse", "Install deckhouse and wait for its readiness.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineKubeFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		installConfig, err := deckhouse.PrepareDeckhouseInstallConfig(metaConfig)
		if err != nil {
			return err
		}

		var sshClient *ssh.Client
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			err = operations.AskBecomePassword()
			if err != nil {
				return err
			}
		}

		return log.Process("bootstrap", "Install Deckhouse", func() error {
			kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
			if err != nil {
				return err
			}

			if err := operations.InstallDeckhouse(kubeCl, installConfig, metaConfig.MasterNodeGroupManifest()); err != nil {
				return err
			}
			return nil
		})
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

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = operations.AskBecomePassword()
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
		var resourcesToCreate *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		if resourcesToCreate == nil || len(resourcesToCreate.Items) == 0 {
			log.WarnLn("Resources to create were not found.")
			return nil
		}

		var sshClient *ssh.Client
		var err error
		if app.SSHHost != "" {
			sshClient, err = ssh.NewClientFromFlags().Start()
			if err != nil {
				return err
			}

			err = operations.AskBecomePassword()
			if err != nil {
				return err
			}
		}

		return log.Process("bootstrap", "Create resources", func() error {
			kubeCl, err := operations.ConnectToKubernetesAPI(sshClient)
			if err != nil {
				return err
			}

			return resources.CreateResourcesLoop(kubeCl, resourcesToCreate)
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
	Use "candictl destroy" command to delete the cluster.
`
	bootstrapAbortCheckMessage = `You will be asked for approval multiple times.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
)

func DefineBootstrapAbortCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("abort", "Delete every node, which was created during bootstrap process.")
	app.DefineConfigFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineSanityFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		cachePath := metaConfig.CachePath()
		stateCache, err := cache.NewTempStateCache(cachePath)
		if err != nil {
			return fmt.Errorf(bootstrapAbortInvalidCacheMessage, cachePath, err)
		}

		if !stateCache.InCache("uuid") {
			return fmt.Errorf("No UUID found in the cache. Perhaps, the cluster was already bootstrapped.")
		}

		_ = log.Process("common", "Get cluster UUID from the cache", func() error {
			metaConfig.UUID = string(stateCache.Load("uuid"))
			log.InfoF("Cluster UUID: %s\n", metaConfig.UUID)
			return nil
		})

		nodesToDelete, err := operations.BootstrapGetNodesFromCache(metaConfig, stateCache)
		if err != nil {
			return fmt.Errorf("bootstrap-phase abort preparation: %v", err)
		}

		for nodeGroup, nodeData := range nodesToDelete {
			if nodeGroup == "master" {
				// we will destroy masters later because they need additional arguments and different terraform files
				continue
			}

			for index, nodeName := range nodeData {
				nodeRunner := terraform.NewRunnerFromConfig(metaConfig, "static-node").
					WithVariables(metaConfig.NodeGroupConfig(nodeGroup, index, "")).
					WithName(nodeName).
					WithCache(stateCache).
					WithAllowedCachedState(true).
					WithAutoApprove(app.SanityCheck)
				tomb.RegisterOnShutdown(nodeName, nodeRunner.Stop)

				if err := terraform.DestroyPipeline(nodeRunner, nodeName); err != nil {
					return err
				}
			}
		}

		if _, ok := nodesToDelete["master"]; ok {
			for index, nodeName := range nodesToDelete["master"] {
				masterRunner := terraform.NewRunnerFromConfig(metaConfig, "master-node").
					WithVariables(metaConfig.NodeGroupConfig("master", index, "")).
					WithName(nodeName).
					WithCache(stateCache).
					WithAllowedCachedState(true).
					WithAutoApprove(app.SanityCheck)
				tomb.RegisterOnShutdown(nodeName, masterRunner.Stop)

				if err := terraform.DestroyPipeline(masterRunner, nodeName); err != nil {
					return err
				}
			}
		}

		baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
			WithVariables(metaConfig.MarshalConfig()).
			WithCache(stateCache).
			WithAllowedCachedState(true).
			WithAutoApprove(app.SanityCheck)
		tomb.RegisterOnShutdown("base-infrastructure", baseRunner.Stop)

		if err := terraform.DestroyPipeline(baseRunner, "Kubernetes cluster"); err != nil {
			return err
		}

		stateCache.Clean()
		// Allow to reuse cache because cluster will be bootstrapped again (probably)
		stateCache.Delete(".tombstone")
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

		stateCache, err := cache.NewTempStateCache(cachePath)
		if err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(cacheMessage, cachePath, err)
		}

		if app.DropCache {
			stateCache.Clean()
			stateCache.Delete(".tombstone")
		}

		clusterUUID, err := generateClusterUUID()
		if err != nil {
			return err
		}
		metaConfig.UUID = clusterUUID

		return log.Process("bootstrap", "Cloud infrastructure", func() error {
			baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
				WithCache(stateCache).
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
