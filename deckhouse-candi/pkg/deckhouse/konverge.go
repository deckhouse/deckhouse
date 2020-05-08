package deckhouse

import (
	"encoding/json"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/terraform"
)

func RunKonverge(client *kube.KubernetesClient, basePipeline *terraform.Pipeline) error {
	clusterConfig, err := client.CoreV1().Secrets("kube-system").Get(
		"d8-cluster-configuration", metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	var clusterConfigData map[string]json.RawMessage
	err = yaml.Unmarshal(clusterConfig.Data["cluster-configuration.yaml"], &clusterConfigData)
	if err != nil {
		return err
	}

	var clusterConfigDataSpec config.ClusterConfigSpec
	err = json.Unmarshal(clusterConfigData["spec"], &clusterConfigDataSpec)
	if err != nil {
		return err
	}

	if clusterConfigDataSpec.ClusterType == "Static" {
		log.Info("Static cluster does not require further actions")
		return nil
	}

	providerClusterConfig, err := client.CoreV1().Secrets("kube-system").Get(
		"d8-provider-cluster-configuration", metav1.GetOptions{},
	)
	if err != nil {
		return err
	}

	var providerClusterConfigData map[string]json.RawMessage
	err = yaml.Unmarshal(providerClusterConfig.Data["provider-cluster-configuration.yaml"], &providerClusterConfigData)
	if err != nil {
		return err
	}

	basePipeline.MetaConfig.ClusterConfig = clusterConfigData
	basePipeline.MetaConfig.ProviderClusterConfig = providerClusterConfigData

	basePipelineResult, err := basePipeline.Run()
	if err != nil {
		return err
	}

	_, err = client.CoreV1().Secrets("kube-system").Update(
		generateSecretWithTerraformState(basePipelineResult["terraformState"]),
	)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().Secrets("kube-system").Update(generateSecretWithProviderClusterConfig(
		providerClusterConfig.Data["provider-cluster-configuration.yaml"],
		basePipelineResult["cloudDiscovery"],
	))
	if err != nil {
		return err
	}

	return nil
}
