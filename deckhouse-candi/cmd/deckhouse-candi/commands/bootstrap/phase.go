package bootstrap

import (
	"fmt"
	"os"

	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/task"
	"flant/deckhouse-candi/pkg/template"
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

		clusterConfig, err := metaConfig.MarshalClusterConfigYAML()
		if err != nil {
			return fmt.Errorf("marshal cluster config: %v", err)
		}

		providerClusterConfig, err := metaConfig.MarshalProviderClusterConfigYAML()
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

		if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		kubeCl, err := task.StartKubernetesAPIProxy(sshClient)
		if err != nil {
			return err
		}
		if err := task.InstallDeckhouse(kubeCl, &installConfig, metaConfig.MarshalMasterNodeGroupConfig()); err != nil {
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

		err = logboek.LogProcess("⛵ ~ Bootstrap Phase: Install Deckhouse",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })
		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
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

		if err := task.WaitForSSHConnectionOnMaster(sshClient); err != nil {
			return err
		}
		bundleName, err := task.DetermineBundleName(sshClient)
		if err != nil {
			return err
		}

		templateController := template.NewTemplateController("")
		logboek.LogInfoF("Templates Dir: %q\n\n", templateController.TmpDir)

		if err := task.BootstrapMaster(sshClient, bundleName, app.InternalNodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err = task.PrepareBashibleBundle(bundleName, app.InternalNodeIP, metaConfig, templateController); err != nil {
			return err
		}
		if err := task.ExecuteBashibleBundle(sshClient, templateController.TmpDir); err != nil {
			return err
		}
		if err := task.RebootMaster(sshClient); err != nil {
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

		err = logboek.LogProcess("⛵ ~ Bootstrap Phase: Execute bashible bundle",
			log.MainProcessOptions(), func() error { return runFunc(sshClient) })
		if err != nil {
			logboek.LogErrorF("\nCritical Error: %s\n", err)
			os.Exit(1)
		}
		return nil
	})

	return cmd
}
