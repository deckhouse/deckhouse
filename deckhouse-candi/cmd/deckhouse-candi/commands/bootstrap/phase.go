package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/commands"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions/deckhouse"
	"flant/deckhouse-candi/pkg/kubernetes/actions/resources"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/system/ssh"
	"flant/deckhouse-candi/pkg/template"
	"flant/deckhouse-candi/pkg/terraform"
	"flant/deckhouse-candi/pkg/util/cache"
)

func DefineBootstrapInstallDeckhouseCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("install-deckhouse", "Install deckhouse and wait for its readiness.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		// Load deckhouse config
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

		kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}
		if err := commands.InstallDeckhouse(kubeCl, &installConfig, metaConfig.MasterNodeGroupManifest()); err != nil {
			return err
		}
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Install Deckhouse", func() error { return runFunc(sshClient) })
		if err != nil {
			log.ErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}

func DefineBootstrapExecuteBashibleCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("execute-bashible-bundle", "Prepare Master node and install kubernetes.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineInternalNodeAddressFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
		metaConfig, err := config.ParseConfig(app.ConfigPath)
		if err != nil {
			return err
		}

		if err := commands.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		bundleName, err := commands.DetermineBundleName(sshClient)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController("")
		log.InfoF("Templates Dir: %q\n\n", templateController.TmpDir)

		if err := commands.BootstrapMaster(sshClient, bundleName, app.InternalNodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err = commands.PrepareBashibleBundle(bundleName, app.InternalNodeIP, "", metaConfig, templateController); err != nil {
			return err
		}
		if err := commands.ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
			return err
		}
		if err := commands.RebootMaster(sshClient); err != nil {
			return err
		}
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Execute bashible bundle", func() error { return runFunc(sshClient) })
		if err != nil {
			log.ErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}

func DefineCreateResourcesCommand(parent *kingpin.CmdClause) *kingpin.CmdClause {
	cmd := parent.Command("create-resources", "Create resources in Kubernetes cluster.")
	app.DefineSshFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineResourcesFlags(cmd)

	runFunc := func(sshClient *ssh.SshClient) error {
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

		if err := commands.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		kubeCl, err := commands.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}

		return resources.CreateResourcesLoop(kubeCl, resourcesToCreate)
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		sshClient, err := ssh.NewClientFromFlags().Start()
		if err != nil {
			return err
		}

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

		err = log.Process("bootstrap", "Create resources", func() error { return runFunc(sshClient) })
		if err != nil {
			log.ErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
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

			if strings.HasPrefix(info.Name(), "base-infrastructure") || strings.HasPrefix(info.Name(), "uuid") {
				return nil
			}

			name := strings.TrimSuffix(info.Name(), ".tfstate")
			if !r.Match([]byte(name)) {
				return fmt.Errorf(
					"Static nodes state are found in cache\n\t%s\n\t"+
						"It looks like you already have the Kuberenetes cluster."+
						"Please use \"deckhouse-candi destroy\" command to delete the cluster or "+
						"\"deckhouse-candi converge\" command to delete unwanted static nodes.",
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
			cache.Global().AddToClean("base-infrastructure")
			return baseRunner.Destroy()
		})
		if err != nil {
			return err
		}

		cache.Global().AddToClean("uuid")
		cache.Global().Clean()
		return nil
	}

	cmd.Action(func(c *kingpin.ParseContext) error {
		if !app.SanityCheck {
			log.Warning("You will be asked for approve multiple times.\n" +
				"If you understand what you are doing, you can use flag " +
				"--yes-i-am-sane-and-i-understand-what-i-am-doing to skip approvals.\n\n")
		}

		err := log.Process("bootstrap", "Abort", func() error { return runFunc() })
		if err != nil {
			log.ErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
