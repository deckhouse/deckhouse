package deckhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/flant/logboek"
	"github.com/iancoleman/strcase"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kubernetes/actions"
	"flant/deckhouse-candi/pkg/kubernetes/actions/manifests"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/log"
	"flant/deckhouse-candi/pkg/util/retry"
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
	NodesTerraformState   map[string][]byte
	CloudDiscovery        []byte
	DeckhouseConfig       map[string]interface{}
}

func (c *Config) GetImage() string {
	registryNameTemplate := "%s/dev:%s"
	tag := c.DevBranch
	if c.ReleaseChannel != "" {
		registryNameTemplate = "%s:%s"
		tag = strcase.ToKebab(c.ReleaseChannel)
	}
	return fmt.Sprintf(registryNameTemplate, c.Registry, tag)
}

func (c *Config) IsRegistryAccessRequired() bool {
	return c.DockerCfg != ""
}

func deckhouseDeploymentFromConfig(cfg *Config) *appsv1.Deployment {
	return manifests.DeckhouseDeployment(cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired())
}

func CreateDeckhouseManifests(kubeCl *client.KubernetesClient, cfg *Config) error {
	tasks := []actions.ManifestTask{
		{
			Name:     `Namespace "d8-system"`,
			Manifest: func() interface{} { return manifests.DeckhouseNamespace("d8-system") },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Namespaces().Create(manifest.(*apiv1.Namespace))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Namespaces().Update(manifest.(*apiv1.Namespace))
				return err
			},
		},
		{
			Name:     `Admin ClusterRole "cluster-admin"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRole() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Create(manifest.(*rbacv1.ClusterRole))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Update(manifest.(*rbacv1.ClusterRole))
				return err
			},
		},
		{
			Name:     `ClusterRoleBinding "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRoleBinding() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Create(manifest.(*rbacv1.ClusterRoleBinding))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Update(manifest.(*rbacv1.ClusterRoleBinding))
				return err
			},
		},
		{
			Name:     `ServiceAccount "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseServiceAccount() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Create(manifest.(*apiv1.ServiceAccount))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Update(manifest.(*apiv1.ServiceAccount))
				return err
			},
		},
		{
			Name:     `ConfigMap "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseConfigMap(cfg.DeckhouseConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").Create(manifest.(*apiv1.ConfigMap))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").Update(manifest.(*apiv1.ConfigMap))
				return err
			},
		},
	}

	if cfg.IsRegistryAccessRequired() {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "deckhouse-registry"`,
			Manifest: func() interface{} { return manifests.DeckhouseRegistrySecret(cfg.DockerCfg) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.TerraformState) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-terraform-state"`,
			Manifest: func() interface{} { return manifests.SecretWithTerraformState(cfg.TerraformState) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	for nodeName, tfState := range cfg.NodesTerraformState {
		getManifest := func() interface{} { return manifests.SecretWithNodeTerraformState(nodeName, "master", tfState) }
		tasks = append(tasks, actions.ManifestTask{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.ClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() interface{} { return manifests.SecretWithClusterConfig(cfg.ClusterConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	if len(cfg.ProviderClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Secret "d8-provider-cluster-configuration"`,
			Manifest: func() interface{} {
				return manifests.SecretWithProviderClusterConfig(
					cfg.ProviderClusterConfig, cfg.CloudDiscovery,
				)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	tasks = append(tasks, actions.ManifestTask{
		Name: `Deployment "deckhouse"`,
		Manifest: func() interface{} {
			return deckhouseDeploymentFromConfig(cfg)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Create(manifest.(*appsv1.Deployment))
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Update(manifest.(*appsv1.Deployment))
			return err
		},
	})

	return logboek.LogProcess("Create Manifests", log.BoldOptions(), func() error {
		for _, task := range tasks {
			err := task.Create()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func WaitForReadiness(kubeCl *client.KubernetesClient, cfg *Config) error {
	return logboek.LogProcess("Waiting for Deckhouse to become Ready", log.BoldOptions(), func() error {
		// watch for deckhouse pods in namespace become Ready
		ready := make(chan struct{}, 1)

		informer := client.NewDeploymentInformer(context.Background(), kubeCl)
		informer.Namespace = "d8-system"
		informer.FieldSelector = "metadata.name=deckhouse"

		err := informer.CreateSharedInformer()
		if err != nil {
			return err
		}
		defer informer.Stop()

		var waitErr error
		informer.WithKubeEventCb(func(obj *appsv1.Deployment, event string) {
			switch event {
			case "Added":
				fallthrough
			case "Modified":
				// Naive simple ready indicator
				status := obj.Status
				if status.Replicas > 0 && status.Replicas == status.ReadyReplicas && status.UnavailableReplicas == 0 {
					ready <- struct{}{}
				}
			case "Deleted":
				waitErr = fmt.Errorf("deckhouse deployment was deleted while waiting for readiness")
				ready <- struct{}{}
			}
		})

		go func() {
			informer.Run()
		}()

		waitTimer := time.NewTicker(11 * time.Minute)
		defer waitTimer.Stop()
		checkTimer := time.NewTicker(5 * time.Second)
		defer checkTimer.Stop()

		stopLogsChan := make(chan struct{})
		defer func() { stopLogsChan <- struct{}{} }()

		go func() {
			for i := 1; i < 60; i++ {
				time.Sleep(15 * time.Second)
				err = PrintDeckhouseLogs(kubeCl, &stopLogsChan)
				if err != nil {
					logboek.LogInfoLn(err.Error())
					continue
				}
				return
			}
		}()

		for {
			select {
			case <-checkTimer.C:
				continue
			case <-waitTimer.C:
				waitErr = fmt.Errorf("timeout while waiting for deckhouse deployment readiness. Check deckhouse queue and logs for errors")
			case <-ready:
				logboek.LogInfoF("Deckhouse deployment is ready\n")
			}
			break
		}
		return waitErr
	})
}

func DeleteDeckhouseDeployment(kubeCl *client.KubernetesClient) error {
	return logboek.LogProcess("Remove deckhouse", log.BoldOptions(), func() error {
		logboek.LogInfoF("Delete Deployment/deckhouse\n")
		err := kubeCl.AppsV1().Deployments("d8-system").Delete("deckhouse", &metav1.DeleteOptions{})
		if err != nil {
			logboek.LogWarnF("Error: %v\n", err)
		}

		return nil
	})
}

func CreateDeckhouseDeployment(kubeCl *client.KubernetesClient, cfg *Config) error {
	task := actions.ManifestTask{

		Name: `Deployment "deckhouse"`,
		Manifest: func() interface{} {
			return manifests.DeckhouseDeployment(cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired())
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Create(manifest.(*appsv1.Deployment))
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Update(manifest.(*appsv1.Deployment))
			return err
		},
	}

	return logboek.LogProcess("Create Deployment", log.BoldOptions(), func() error {
		return task.Create()
	})
}

func CreateDeckhouseDeploymentManifest(cfg *Config) *appsv1.Deployment {
	return manifests.DeckhouseDeployment(cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired())
}

func WaitForKubernetesAPI(kubeCl *client.KubernetesClient) error {
	return retry.StartLoop("Waiting for Kubernetes API to become Ready", 45, 5, func() error {
		_, err := kubeCl.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
		if err == nil {
			return nil
		}
		return fmt.Errorf("kubernetes API is not Ready: %w", err)
	})
}

func PrepareDeckhouseInstallConfig(metaConfig *config.MetaConfig) (*Config, error) {
	clusterConfig, err := metaConfig.MarshalClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal cluster config: %v", err)
	}

	providerClusterConfig, err := metaConfig.MarshalProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal provider config: %v", err)
	}

	installConfig := Config{
		Registry:              metaConfig.DeckhouseConfig.ImagesRepo,
		DockerCfg:             metaConfig.DeckhouseConfig.RegistryDockerCfg,
		DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
		ReleaseChannel:        metaConfig.DeckhouseConfig.ReleaseChannel,
		Bundle:                metaConfig.DeckhouseConfig.Bundle,
		LogLevel:              metaConfig.DeckhouseConfig.LogLevel,
		DeckhouseConfig:       metaConfig.MergeDeckhouseConfig(),
		ClusterConfig:         clusterConfig,
		ProviderClusterConfig: providerClusterConfig,
	}

	return &installConfig, nil
}
