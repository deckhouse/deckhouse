package commands

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"

	"flant/deckhouse-cluster/pkg/app"
	"flant/deckhouse-cluster/pkg/config"
	"flant/deckhouse-cluster/pkg/deckhouse"
	"flant/deckhouse-cluster/pkg/kube"
	"flant/deckhouse-cluster/pkg/terraform"
)

func GetBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	return kpApp.Command("bootstrap", "Bootstrap cluster.").
		Action(func(c *kingpin.ParseContext) error {
			metaConfig, err := config.ParseConfig(app.ConfigPath)
			if err != nil {
				return err
			}
			metaConfig.PrepareBootstrapSettings()

			clusterConfig, _ := metaConfig.MarshalClusterConfigYAML()
			providerClusterConfig, _ := metaConfig.MarshalProviderClusterConfigYAML()

			installConfig := deckhouse.Config{
				Registry:              metaConfig.BootstrapConfig.Deckhouse.ImagesRepo,
				DockerCfg:             metaConfig.BootstrapConfig.Deckhouse.RegistryDockerCfg,
				Bundle:                metaConfig.BootstrapConfig.Deckhouse.Bundle,
				LogLevel:              metaConfig.BootstrapConfig.Deckhouse.LogLevel,
				ClusterConfig:         clusterConfig,
				ProviderClusterConfig: providerClusterConfig,
			}

			if metaConfig.ClusterType == "Cloud" {
				basePipelineResult, err := terraform.NewPipeline("tf_base", metaConfig, terraform.GetBasePipelineResult).Run()
				if err != nil {
					return err
				}

				masterPipelineResult, err := terraform.NewPipeline("tf_master", metaConfig, terraform.GetMasterPipelineResult).Run()
				if err != nil {
					return err
				}

				installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig(
					basePipelineResult["deckhouseConfig"],
					masterPipelineResult["deckhouseConfig"],
				)
				installConfig.CloudDiscovery = basePipelineResult["cloudDiscovery"]
				installConfig.TerraformState = basePipelineResult["terraformState"]

				log.Infof("Master IP: %s", masterPipelineResult["masterIP"])
				log.Infof("Deckhouse Merged Config: %v", installConfig.DeckhouseConfig)
				log.Infof("Master Instance Group: %s", string(masterPipelineResult["masterInstanceClass"]))
			} else {
				installConfig.DeckhouseConfig = metaConfig.MergeDeckhouseConfig()
			}
			// Generate bashible bundle

			// Upload bundle and run it

			// Open connection to kubernetes API
			kubeCl := kube.NewKubernetesClient()
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
}
