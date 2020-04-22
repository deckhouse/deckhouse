package commands

import (
	"encoding/json"
	"fmt"
	"github.com/flant/logboek"
	"gopkg.in/alecthomas/kingpin.v2"
	"os/exec"
	"strings"

	"flant/deckhouse-candi/pkg/app"
	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/deckhouse"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/ssh"
	"flant/deckhouse-candi/pkg/template"
	"flant/deckhouse-candi/pkg/terraform"
)

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSshFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.IsDebug = 1

	cmd.Action(func(c *kingpin.ParseContext) error {
		logboek.LogInfoLn("Starting cluster bootstrap process...")

		// Start ssh-agent and ask passwords for keys
		sshClient, err := ssh.NewClientFromFlags().StartSession()
		if err != nil {
			return err
		}
		defer sshClient.StopSession()

		err = app.AskBecomePassword()
		if err != nil {
			return err
		}

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
		}

		var nodeIP string
		if metaConfig.ClusterType == "Cloud" {
			basePipelineResult, err := terraform.NewPipeline(
				"base_infrastructure",
				metaConfig,
				terraform.GetBasePipelineResult,
			).Run()
			if err != nil {
				return err
			}

			masterPipelineResult, err := terraform.NewPipeline(
				"master_node_bootstrap",
				metaConfig,
				terraform.GetMasterPipelineResult,
			).Run()
			if err != nil {
				return err
			}

			installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig(
				basePipelineResult["deckhouseConfig"],
				masterPipelineResult["deckhouseConfig"],
			)
			installConfig.CloudDiscovery = basePipelineResult["cloudDiscovery"]
			installConfig.TerraformState = basePipelineResult["terraformState"]

			_ = json.Unmarshal(masterPipelineResult["masterIP"], &app.SshHost)
			_ = json.Unmarshal(masterPipelineResult["nodeIP"], &nodeIP)

			sshClient.Session.Host = app.SshHost

			app.Debugf("Master IP: %s", masterPipelineResult["masterIP"])
			app.Debugf("Deckhouse Merged Config: %v", installConfig.DeckhouseConfig)
			app.Debugf("Master Instance Group: %s", string(masterPipelineResult["masterInstanceClass"]))
		} else {
			installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig()
		}
		// Generate bashible bundle

		// wait for ssh connection to master
		err = sshClient.Check().AwaitAvailability()
		if err != nil {
			return fmt.Errorf("await master available: %v", err)
		}

		// run detect bundle type
		detectCmd := sshClient.UploadScript("/deckhouse/candi/bashible/detect_bundle.sh")
		stdout, err := detectCmd.Execute()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("script '%s' error: %v\nstderr: %s", "detect_bundle.sh", err, string(ee.Stderr))
			}
			return fmt.Errorf("script '%s' error: %v", "detect_bundle.sh", err)
		}

		bundleName := strings.Trim(string(stdout), "\n ")
		logboek.LogInfoF("\n\nDetected bundle: %s\n\n", bundleName)

		// Generate bootstrap scripts
		templateController := template.NewTemplateController("")
		logboek.LogInfoF("Templates Dir: %q\n", templateController.TmpDir)

		bashibleData := metaConfig.MarshalConfigForBashibleBundleTemplate(bundleName, nodeIP)

		if err := templateController.RenderAndSaveTemplates(
			"/deckhouse/candi/bashible/bundles/"+bundleName,
			"/bootstrap/",
			bashibleData,
		); err != nil {
			return err
		}

		if err := templateController.RenderAndSaveTemplates(
			fmt.Sprintf("/deckhouse/candi/cloud-providers/%s/bashible-bundles/%s", metaConfig.ProviderName, bundleName),
			"/bootstrap/",
			bashibleData,
		); err != nil {
			return err
		}

		// Run Bootstrap
		for _, bootstrapScript := range []string{"bootstrap.sh", "bootstrap-networks.sh"} {
			logboek.LogProcessStart("Execute bootstrap "+bootstrapScript, logboek.LogProcessStartOptions{})

			cmd := sshClient.UploadScript(templateController.TmpDir + "/bootstrap/" + bootstrapScript).Sudo()

			stdout, err := cmd.Execute()
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return fmt.Errorf("script 'bootstrap/%s' error: %v\nstderr: %s", bootstrapScript, err, string(ee.Stderr))
				}
				return fmt.Errorf("script 'bootstrap/%s' error: %v", bootstrapScript, err)
			}
			logboek.LogInfoF("bootstrap/%s stdout: %v\n", bootstrapScript, string(stdout))
			logboek.LogProcessEnd(logboek.LogProcessEndOptions{})
		}
		// defer templateController.Close()

		// Generate bundle
		if err = template.PrepareBundle(templateController, nodeIP, bundleName, metaConfig); err != nil {
			return fmt.Errorf("prepare bundle: %v", err)
		}

		bundleCmd := sshClient.UploadScript("bashible.sh", "--local").Sudo()
		parentDir := templateController.TmpDir + "/var/lib"
		bundleDir := "bashible"

		stdout, err = bundleCmd.ExecuteBundle(parentDir, bundleDir)
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return fmt.Errorf("bundle '%s' error: %v\nstderr: %s", bundleDir, err, string(ee.Stderr))
			}
			return fmt.Errorf("bundle '%s' error: %v", bundleDir, err)
		}
		logboek.LogInfoF("Got %d symbols\n", len(stdout))
		// Upload bundle and run it

		// Open connection to kubernetes API
		kubeCl := kube.NewKubernetesClient().WithSshClient(sshClient)
		// auto init
		err = kubeCl.Init("")
		if err != nil {
			return fmt.Errorf("open kubernetes connection: %v", err)
		}
		// defer stop ssh-agent, proxy and a tunnel
		defer kubeCl.Stop()

		// Install Deckhouse
		_ = deckhouse.CreateDeckhouseManifests(kubeCl, &installConfig)

		return nil
	})

	return cmd
}
