package deckhouse

import (
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"

	"flant/deckhouse-candi/pkg/kube"
)

type Config struct {
	Registry              string
	DockerCfg             string
	LogLevel              string
	Bundle                string
	ReleaseChannel        string
	DevBranch             string
	ClusterConfig         []byte
	ProviderClusterConfig []byte
	TerraformState        []byte
	CloudDiscovery        []byte
	DeckhouseConfig       map[string]interface{}
}

func (c *Config) IsRegistryAccessRequired() bool {
	return c.DockerCfg != ""
}

func CreateDeckhouseManifests(client *kube.KubernetesClient, cfg *Config) error {
	_, err := client.CoreV1().Namespaces().Create(generateDeckhouseNamespace("d8-system"))
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`Namespace "d8-system" already exists`)
		return err
	}

	_, err = client.RbacV1().ClusterRoles().Create(generateDeckhouseAdminClusterRole())
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`Admin ClusterRole "cluster-admin" already exists`)
		return err
	}
	log.Infof(`Admin ClusterRole "cluster-admin" created`)

	_, err = client.RbacV1().ClusterRoleBindings().Create(generateDeckhouseAdminClusterRoleBinding())
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`ClusterRoleBinding "deckhouse" already exists`)
		return err
	}
	log.Infof(`ClusterRoleBinding "deckhouse" created`)

	_, err = client.CoreV1().ServiceAccounts("d8-system").Create(generateDeckhouseServiceAccount())
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`ServiceAccount "deckhouse" already exists`)
		return err
	}
	log.Infof(`ServiceAccount "deckhouse" created`)

	if cfg.IsRegistryAccessRequired() {
		_, err = client.CoreV1().Secrets("d8-system").Create(generateDeckhouseRegistrySecret(cfg.DockerCfg))
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Infof(`Secret "deckhouse-registry" already exists`)
			return err
		}
		log.Infof(`Secret "deckhouse-registry" created`)
	}

	configMap, err := generateDeckhouseConfigMap(cfg.DeckhouseConfig)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ConfigMaps("d8-system").Create(configMap)
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`ConfigMap "deckhouse" already exists`)
		return err
	}
	log.Infof(`ConfigMap "deckhouse" created`)

	image := cfg.Registry + ":" + strcase.ToKebab(cfg.ReleaseChannel)
	if cfg.ReleaseChannel == "" {
		image = cfg.Registry + "/dev:" + cfg.DevBranch
	}

	_, err = client.AppsV1().Deployments("d8-system").Create(generateDeckhouseDeployment(
		image, cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired(),
	))
	if err != nil && !errors.IsAlreadyExists(err) {
		log.Infof(`Deployment "deckhouse" already exists`)
		return err
	}
	log.Infof(`Deployment "deckhouse" created`)

	if len(cfg.TerraformState) > 0 {
		_, err = client.CoreV1().Secrets("kube-system").Create(generateSecretWithTerraformState(cfg.TerraformState))
		if err != nil && !errors.IsAlreadyExists(err) {
			log.Infof(`Secret "d8-terraform-state" already exists`)
			return err
		}
		log.Infof(`Secret "d8-terraform-state" created`)
	}

	if len(cfg.ClusterConfig) > 0 {
		_, err = client.CoreV1().Secrets("kube-system").Create(generateSecretWithClusterConfig(cfg.ClusterConfig))

		if err != nil && !errors.IsAlreadyExists(err) {
			log.Infof(`Secret "d8-cluster-configuration" already exists`)
			return err
		}
		log.Infof(`Secret "d8-cluster-configuration" created`)
	}

	if len(cfg.ProviderClusterConfig) > 0 {
		_, err = client.CoreV1().Secrets("kube-system").Create(
			generateSecretWithProviderClusterConfig(cfg.ProviderClusterConfig, cfg.CloudDiscovery),
		)

		if err != nil && !errors.IsAlreadyExists(err) {
			log.Infof(`Secret "d8-provider-cluster-configuration" already exists`)
			return err
		}
		log.Infof(`Secret "d8-provider-cluster-configuration" created`)
	}
	return nil
}
