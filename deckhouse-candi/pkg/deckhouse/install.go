package deckhouse

import (
	"github.com/flant/logboek"
	"github.com/iancoleman/strcase"
	"k8s.io/apimachinery/pkg/api/errors"

	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
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
	return logboek.LogProcess("Create Manifests", log.BoldOptions(), func() error {
		logboek.LogInfoLn(`Create Namespace "d8-system"`)
		_, err := client.CoreV1().Namespaces().Create(generateDeckhouseNamespace("d8-system"))
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`Namespace "d8-system" already exists`)
		}

		logboek.LogInfoLn(`Create Admin ClusterRole "cluster-admin" `)
		_, err = client.RbacV1().ClusterRoles().Create(generateDeckhouseAdminClusterRole())
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`Admin ClusterRole "cluster-admin" already exists`)
		}

		logboek.LogInfoLn(`Create ClusterRoleBinding "deckhouse"`)
		_, err = client.RbacV1().ClusterRoleBindings().Create(generateDeckhouseAdminClusterRoleBinding())
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`ClusterRoleBinding "deckhouse" already exists`)
		}

		logboek.LogInfoLn(`Create ServiceAccount "deckhouse"`)
		_, err = client.CoreV1().ServiceAccounts("d8-system").Create(generateDeckhouseServiceAccount())
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`ServiceAccount "deckhouse" already exists`)
		}

		if cfg.IsRegistryAccessRequired() {
			logboek.LogInfoLn(`Create Secret "deckhouse-registry"`)

			_, err = client.CoreV1().Secrets("d8-system").Create(generateDeckhouseRegistrySecret(cfg.DockerCfg))
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogWarnLn(`Secret "deckhouse-registry" already exists`)
			}
		}

		configMap, err := generateDeckhouseConfigMap(cfg.DeckhouseConfig)
		if err != nil {
			return err
		}

		logboek.LogInfoLn(`Create ConfigMap "deckhouse"`)
		_, err = client.CoreV1().ConfigMaps("d8-system").Create(configMap)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`ConfigMap "deckhouse" already exists`)
		}

		image := cfg.Registry + ":" + strcase.ToKebab(cfg.ReleaseChannel)
		if cfg.ReleaseChannel == "" {
			image = cfg.Registry + "/dev:" + cfg.DevBranch
		}

		logboek.LogInfoLn(`Create Deployment "deckhouse" created`)
		_, err = client.AppsV1().Deployments("d8-system").Create(generateDeckhouseDeployment(
			image, cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired(),
		))
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				return err
			}
			logboek.LogWarnLn(`Deployment "deckhouse" already exists`)
		}

		if len(cfg.TerraformState) > 0 {
			logboek.LogInfoLn(`Create Secret "d8-terraform-state"`)
			_, err = client.CoreV1().Secrets("kube-system").Create(generateSecretWithTerraformState(cfg.TerraformState))
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogWarnLn(`Secret "d8-terraform-state" already exists`)
			}
		}

		if len(cfg.ClusterConfig) > 0 {
			logboek.LogInfoLn(`Create Secret "d8-cluster-configuration"`)
			_, err = client.CoreV1().Secrets("kube-system").Create(generateSecretWithClusterConfig(cfg.ClusterConfig))
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogWarnLn(`Secret "d8-cluster-configuration" already exists`)
			}
		}

		if len(cfg.ProviderClusterConfig) > 0 {
			logboek.LogInfoLn(`Create Secret "d8-provider-cluster-configuration"`)
			_, err = client.CoreV1().Secrets("kube-system").Create(
				generateSecretWithProviderClusterConfig(cfg.ProviderClusterConfig, cfg.CloudDiscovery),
			)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogWarnLn(`Secret "d8-provider-cluster-configuration" already exists`)
			}
		}
		return nil
	})
}
