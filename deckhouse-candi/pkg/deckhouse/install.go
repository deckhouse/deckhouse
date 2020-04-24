package deckhouse

import (
	"github.com/flant/logboek"
	"github.com/iancoleman/strcase"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

type createManifestTask struct {
	name       string
	createTask func(manifest interface{}) error
	updateTask func(manifest interface{}) error
	manifest   func() interface{}
}

func CreateDeckhouseManifests(client *kube.KubernetesClient, cfg *Config) error {
	image := cfg.Registry + ":" + strcase.ToKebab(cfg.ReleaseChannel)
	if cfg.ReleaseChannel == "" {
		image = cfg.Registry + "/dev:" + cfg.DevBranch
	}

	tasks := []createManifestTask{
		{
			name:     `Namespace "d8-system"`,
			manifest: func() interface{} { return generateDeckhouseNamespace("d8-system") },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Namespaces().Create(manifest.(*apiv1.Namespace))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Namespaces().Update(manifest.(*apiv1.Namespace))
				return err
			},
		},
		{
			name:     `Admin ClusterRole "cluster-admin"`,
			manifest: func() interface{} { return generateDeckhouseAdminClusterRole() },
			createTask: func(manifest interface{}) error {
				_, err := client.RbacV1().ClusterRoles().Create(manifest.(*rbacv1.ClusterRole))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.RbacV1().ClusterRoles().Update(manifest.(*rbacv1.ClusterRole))
				return err
			},
		},
		{
			name:     `ClusterRoleBinding "deckhouse"`,
			manifest: func() interface{} { return generateDeckhouseAdminClusterRoleBinding() },
			createTask: func(manifest interface{}) error {
				_, err := client.RbacV1().ClusterRoleBindings().Create(manifest.(*rbacv1.ClusterRoleBinding))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.RbacV1().ClusterRoleBindings().Update(manifest.(*rbacv1.ClusterRoleBinding))
				return err
			},
		},
		{
			name:     `ServiceAccount "deckhouse"`,
			manifest: func() interface{} { return generateDeckhouseServiceAccount() },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().ServiceAccounts("d8-system").Create(manifest.(*apiv1.ServiceAccount))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().ServiceAccounts("d8-system").Update(manifest.(*apiv1.ServiceAccount))
				return err
			},
		},
		{
			name:     `ConfigMap "deckhouse"`,
			manifest: func() interface{} { return generateDeckhouseConfigMap(cfg.DeckhouseConfig) },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().ConfigMaps("d8-system").Create(manifest.(*apiv1.ConfigMap))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().ConfigMaps("d8-system").Update(manifest.(*apiv1.ConfigMap))
				return err
			},
		},
		{
			name: `Deployment "deckhouse"`,
			manifest: func() interface{} {
				return generateDeckhouseDeployment(
					image, cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired(),
				)
			},
			createTask: func(manifest interface{}) error {
				_, err := client.AppsV1().Deployments("d8-system").Create(manifest.(*appsv1.Deployment))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.AppsV1().Deployments("d8-system").Update(manifest.(*appsv1.Deployment))
				return err
			},
		},
	}

	if cfg.IsRegistryAccessRequired() {
		tasks = append(tasks, createManifestTask{
			name:     `Secret "deckhouse-registry"`,
			manifest: func() interface{} { return generateDeckhouseRegistrySecret(cfg.DockerCfg) },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.TerraformState) > 0 {
		tasks = append(tasks, createManifestTask{
			name:     `Secret "d8-terraform-state"`,
			manifest: func() interface{} { return generateSecretWithTerraformState(cfg.TerraformState) },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.ClusterConfig) > 0 {
		tasks = append(tasks, createManifestTask{
			name:     `Secret "d8-cluster-configuration"`,
			manifest: func() interface{} { return generateSecretWithClusterConfig(cfg.ClusterConfig) },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.ProviderClusterConfig) > 0 {
		tasks = append(tasks, createManifestTask{
			name: `Secret "d8-provider-cluster-configuration"`,
			manifest: func() interface{} {
				return generateSecretWithProviderClusterConfig(
					cfg.ProviderClusterConfig, cfg.CloudDiscovery,
				)
			},
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("kube-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	return logboek.LogProcess("Create Manifests", log.BoldOptions(), func() error {
		for _, task := range tasks {
			logboek.LogInfoF("Create %s\n", task.name)
			manifest := task.manifest()

			err := task.createTask(manifest)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogInfoF("%s already exists. Trying to update ... ", task.name)
				err = task.updateTask(manifest)
				if err != nil {
					logboek.LogInfoLn("ERROR!")
					return err
				}
				logboek.LogInfoLn("OK!")
			}
		}
		return nil
	})
}
