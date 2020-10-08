package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/candictl/pkg/app"
	"flant/candictl/pkg/config"
	"flant/candictl/pkg/kubernetes/actions/deckhouse"
	"flant/candictl/pkg/kubernetes/actions/resources"
	"flant/candictl/pkg/log"
	"flant/candictl/pkg/operations"
	"flant/candictl/pkg/system/ssh"
	"flant/candictl/pkg/template"
	"flant/candictl/pkg/terraform"
	"flant/candictl/pkg/util/cache"
	"flant/candictl/pkg/util/tomb"
)

func DefineBootstrapInstallDeckhouseCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install-deckhouse", "Install deckhouse and wait for its readiness.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		clusterConfig, err := metaConfig.ClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal cluster config: %v", err)
		}

		providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal provider config: %v", err)
		}

		installConfig := deckhouse.Config{
			Registry:              metaConfig.DeckhouseConfig.ImagesRepo,
			DockerCfg:             metaConfig.DeckhouseConfig.RegistryDockerCfg,
			DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
			ReleaseChannel:        metaConfig.DeckhouseConfig.ReleaseChannel,
			Bundle:                metaConfig.DeckhouseConfig.Bundle,
			LogLevel:              metaConfig.DeckhouseConfig.LogLevel,
			ClusterConfig:         clusterConfig,
			ProviderClusterConfig: providerClusterConfig,
			DeckhouseConfig:       metaConfig.MergeDeckhouseConfig(),
		}

		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = operations.AskBecomePassword()
		if err != nil {
			return err
		}

		kubeCl, err := operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		if err := operations.InstallDeckhouse(kubeCl, &installConfig, metaConfig.MasterNodeGroupManifest()); err != nil {
			return err
		}
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Install Deckhouse", func() error { return runFunc() })
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("execute-bashible-bundle", "Prepare Master node and install Kubernetes.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineInternalNodeAddressFlags(cmd)

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
		bundleName, err := operations.DetermineBundleName(sshClient)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController("")
		log.InfoF("Templates Dir: %q\n\n", templateController.TmpDir)

		if err := operations.BootstrapMaster(sshClient, bundleName, app.InternalNodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err = operations.PrepareBashibleBundle(bundleName, app.InternalNodeIP, "", metaConfig, templateController); err != nil {
			return err
		}
		if err := operations.ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
			return err
		}
		if err := operations.RebootMaster(sshClient); err != nil {
			return err
		}
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Execute bashible bundle", func() error { return runFunc() })
	})

	return cmd
}

func DefineCreateResourcesCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-resources", "Create resources in Kubernetes cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineResourcesFlags(cmd)

	runFunc := func() error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = operations.AskBecomePassword()
		if err != nil {
			return err
		}

		var resourcesToCreate *config.Resources
		if app.ResourcesPath != "" {
			parsedResources, err := config.ParseResources(app.ResourcesPath)
			if err != nil {
				return err
			}

			resourcesToCreate = parsedResources
		}

		if resourcesToCreate == nil {
			return nil
		}

		if err := operations.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		kubeCl, err := operations.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(kubeCl, resourcesToCreate)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		return log.Process("bootstrap", "Create resources", func() error { return runFunc() })
	})

	return cmd
}

func DefineBootstrapAbortCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("abort", "Delete every node, which was created during bootstrap process.")
	app.DefineConfigFlags(cmd)
	app.DefineSanityFlags(cmd)

	runFunc := func() error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		cachePath := metaConfig.CachePath()
		if err = cache.Init(cachePath); err != nil {
			// TODO: it's better to ask for confirmation here
			return fmt.Errorf(
				"Create cache %s:\n\tError: %v\n\n"+
					"\tProbably that Kubernetes cluster was successfully bootstrapped.\n"+
					"\tUse \"dekchouse-candi destroy\" command to delete the cluster.",
				cachePath, err,
			)
		}

		if !cache.Global().InCache("uuid") {
			return fmt.Errorf("No UUID found in cached. Pheraps, the cluster was already bootstrapped.")
		}

		metaConfig.UUID = string(cache.Global().Load("uuid"))
		log.InfoF("Cluster UUID from cache: %s\n", metaConfig.UUID)

		masterGroupRegexp := fmt.Sprintf("^%s-master-([0-9]+)$", metaConfig.ClusterPrefix)
		r, _ := regexp.Compile(masterGroupRegexp)

		nodesToDelete := make(map[string][]byte)
		if err := filepath.Walk(cache.Global().GetDir(), func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(path, ".backup") {
				return nil
			}

			if strings.HasPrefix(info.Name(), "base-infrastructure") || strings.HasPrefix(info.Name(), "uuid") {
				return nil
			}

			name := strings.TrimSuffix(info.Name(), ".tfstate")
			if !r.Match([]byte(name)) {
				return fmt.Errorf(
					"Static nodes state are found in cache\n\t%s\n\t"+
						"It looks like you already have the Kuberenetes cluster. "+
						"Please use \"candictl destroy\" command to delete the cluster or "+
						"\"candictl converge\" command to delete unwanted static nodes.",
					cache.Global().ObjectPath(name),
				)
			}
			nodesToDelete[name] = cache.Global().Load(name)
			return nil
		}); err != nil {
			return fmt.Errorf("can't iterate the cache: %v", err)
		}

		for nodeName, state := range nodesToDelete {
			err := log.Process("terraform", fmt.Sprintf("Destroy Node %s", nodeName), func() error {
				masterRunner := terraform.NewRunnerFromConfig(metaConfig, "master-node").
					WithVariables(metaConfig.NodeGroupConfig("master", 0, "")).
					WithName(nodeName).
					WithState(state).
					WithAutoApprove(app.SanityCheck)
				tomb.RegisterOnShutdown(masterRunner.Stop)

				cache.Global().AddToClean(nodeName)
				return masterRunner.Destroy()
			})
			if err != nil {
				return err
			}
		}

		err = log.Process("terraform", "Destroy base-infrastructure", func() error {
			baseRunner := terraform.NewRunnerFromConfig(metaConfig, "base-infrastructure").
				WithVariables(metaConfig.MarshalConfig()).
				WithState(cache.Global().Load("base-infrastructure")).
				WithAutoApprove(app.SanityCheck)
			tomb.RegisterOnShutdown(baseRunner.Stop)

			cache.Global().AddToClean("base-infrastructure")
			return baseRunner.Destroy()
		})
		if err != nil {
			return err
		}

		cache.Global().AddToClean("uuid")
		cache.Global().Clean()
		cache.Global().Teardown()
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.Warning("You will be asked for approve multiple times.\n" +
				"If you understand what you are doing, you can use flag " +
				"--yes-i-am-sane-and-i-understand-what-i-am-doing to skip approvals.\n\n")
		}

		return log.Process("bootstrap", "Abort", func() error { return runFunc() })
	})

	return cmd
}
