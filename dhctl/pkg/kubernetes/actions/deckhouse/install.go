// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deckhouse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Config struct {
	Registry              config.RegistryData
	LogLevel              string
	Bundle                string
	ReleaseChannel        string
	DevBranch             string
	UUID                  string
	KubeDNSAddress        string
	ClusterConfig         []byte
	ProviderClusterConfig []byte
	StaticClusterConfig   []byte
	TerraformState        []byte
	NodesTerraformState   map[string][]byte
	CloudDiscovery        []byte
	DeckhouseConfig       map[string]interface{}

	KubeadmBootstrap   bool
	MasterNodeSelector bool
}

func (c *Config) GetImage() string {
	registryNameTemplate := "%s%s/dev:%s"
	tag := c.DevBranch
	if c.ReleaseChannel != "" {
		registryNameTemplate = "%s%s:%s"
		tag = strcase.ToKebab(c.ReleaseChannel)
	}
	return fmt.Sprintf(registryNameTemplate, c.Registry.Address, c.Registry.Path, tag)
}

func (c *Config) IsRegistryAccessRequired() bool {
	return c.Registry.DockerCfg != ""
}

func prepareDeckhouseDeploymentForUpdate(kubeCl *client.KubernetesClient, cfg *Config, manifestForUpdate *appsv1.Deployment) (*appsv1.Deployment, error) {
	resDeployment := manifestForUpdate
	err := retry.NewSilentLoop("get deployment", 10, 3*time.Second).Run(func() error {
		currentManifestInCluster, err := kubeCl.AppsV1().Deployments(manifestForUpdate.GetNamespace()).Get(context.TODO(), manifestForUpdate.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Parametrize existing Deployment manifest to prevent redundant restarting
		// of deckhouse's Pod if params are not changed between dhctl executions.
		// deployTime is a 'write once' parameter, so it is preserved for this check.
		//
		// It helps to reduce wait time on bootstrap process restarting,
		// and prevents a race condition when deckhouse's Pod is scheduled
		// on the non-approved node, so the bootstrap process never finishes.
		params := deckhouseDeploymentParamsFromCfg(cfg)
		params.DeployTime = manifests.GetDeckhouseDeployTime(currentManifestInCluster)

		resDeployment = manifests.ParameterizeDeckhouseDeployment(currentManifestInCluster.DeepCopy(), params)

		return nil
	})

	return resDeployment, err
}

func controllerDeploymentTask(kubeCl *client.KubernetesClient, cfg *Config) actions.ManifestTask {
	return actions.ManifestTask{
		Name: `Deployment "deckhouse"`,
		Manifest: func() interface{} {
			return CreateDeckhouseDeploymentManifest(cfg)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Create(context.TODO(), manifest.(*appsv1.Deployment), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			preparedManifest, err := prepareDeckhouseDeploymentForUpdate(kubeCl, cfg, manifest.(*appsv1.Deployment))
			if err != nil {
				return err
			}

			_, err = kubeCl.AppsV1().Deployments("d8-system").Update(context.TODO(), preparedManifest, metav1.UpdateOptions{})

			return err
		},
	}
}

func CreateDeckhouseManifests(kubeCl *client.KubernetesClient, cfg *Config) error {
	tasks := []actions.ManifestTask{
		{
			Name:     `Namespace "d8-system"`,
			Manifest: func() interface{} { return manifests.DeckhouseNamespace("d8-system") },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Namespaces().Create(context.TODO(), manifest.(*apiv1.Namespace), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Namespaces().Update(context.TODO(), manifest.(*apiv1.Namespace), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `Admin ClusterRole "cluster-admin"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRole() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Create(context.TODO(), manifest.(*rbacv1.ClusterRole), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Update(context.TODO(), manifest.(*rbacv1.ClusterRole), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `ClusterRoleBinding "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRoleBinding() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Create(context.TODO(), manifest.(*rbacv1.ClusterRoleBinding), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Update(context.TODO(), manifest.(*rbacv1.ClusterRoleBinding), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `ServiceAccount "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseServiceAccount() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Create(context.TODO(), manifest.(*apiv1.ServiceAccount), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Update(context.TODO(), manifest.(*apiv1.ServiceAccount), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `ConfigMap "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseConfigMap(cfg.DeckhouseConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").Create(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").Update(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		},
	}

	if cfg.IsRegistryAccessRequired() {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "deckhouse-registry"`,
			Manifest: func() interface{} { return manifests.DeckhouseRegistrySecret(cfg.Registry) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		})
	}

	if len(cfg.TerraformState) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-terraform-state"`,
			Manifest: func() interface{} { return manifests.SecretWithTerraformState(cfg.TerraformState) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		})
	}

	for nodeName, tfState := range cfg.NodesTerraformState {
		getManifest := func() interface{} { return manifests.SecretWithNodeTerraformState(nodeName, "master", tfState, nil) }
		tasks = append(tasks, actions.ManifestTask{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		})
	}

	if len(cfg.ClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() interface{} { return manifests.SecretWithClusterConfig(cfg.ClusterConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(context.TODO(), manifest.(*apiv1.Secret), metav1.UpdateOptions{})
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
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(context.TODO(),
					"d8-provider-cluster-configuration",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
				return err
			},
		})
	}

	if len(cfg.StaticClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Secret "d8-static-cluster-configuration"`,
			Manifest: func() interface{} {
				return manifests.SecretWithStaticClusterConfig(cfg.StaticClusterConfig)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Create(context.TODO(), manifest.(*apiv1.Secret), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(
					context.TODO(),
					"d8-static-cluster-configuration",
					types.MergePatchType,
					data,
					metav1.PatchOptions{},
				)
				return err
			},
		})
	}

	if len(cfg.UUID) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name: `ConfigMap "d8-cluster-uuid"`,
			Manifest: func() interface{} {
				return manifests.ClusterUUIDConfigMap(cfg.UUID)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("kube-system").Create(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		})
	}

	if cfg.KubeDNSAddress != "" {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Service "kube-dns"`,
			Manifest: func() interface{} {
				return manifests.KubeDNSService(cfg.KubeDNSAddress)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Services("kube-system").Create(context.TODO(), manifest.(*apiv1.Service), metav1.CreateOptions{})
				if err != nil && strings.Contains(err.Error(), "provided IP is already allocated") {
					log.InfoLn("Service for DNS already exists. Skip!")
					return nil
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Services("kube-system").Update(context.TODO(), manifest.(*apiv1.Service), metav1.UpdateOptions{})
				return err
			},
		})
	}

	tasks = append(tasks, controllerDeploymentTask(kubeCl, cfg))

	return log.Process("default", "Create Manifests", func() error {
		for _, task := range tasks {
			err := task.CreateOrUpdate()
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func WaitForReadiness(kubeCl *client.KubernetesClient) error {
	return WaitForReadinessNotOnNode(kubeCl, "")
}

func WaitForReadinessNotOnNode(kubeCl *client.KubernetesClient, excludeNode string) error {
	return log.Process("default", "Waiting for Deckhouse to become Ready", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), app.DeckhouseTimeout)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return ErrTimedOut
			default:
				ok, err := NewLogPrinter(kubeCl).
					WaitPodBecomeReady().
					WithExcludeNode(excludeNode).
					Print(ctx)

				if err != nil {
					if errors.Is(err, ErrTimedOut) {
						return err
					}
					log.InfoLn(err.Error())
				}

				if ok {
					log.InfoLn("Deckhouse pod is Ready!")
					return nil
				}

				time.Sleep(5 * time.Second)
			}
		}
	})
}

func CreateDeckhouseDeployment(kubeCl *client.KubernetesClient, cfg *Config) error {
	task := controllerDeploymentTask(kubeCl, cfg)

	return log.Process("default", "Create Deployment", task.CreateOrUpdate)
}

func deckhouseDeploymentParamsFromCfg(cfg *Config) manifests.DeckhouseDeploymentParams {
	return manifests.DeckhouseDeploymentParams{
		Registry:           cfg.GetImage(),
		LogLevel:           cfg.LogLevel,
		Bundle:             cfg.Bundle,
		IsSecureRegistry:   cfg.IsRegistryAccessRequired(),
		KubeadmBootstrap:   cfg.KubeadmBootstrap,
		MasterNodeSelector: cfg.MasterNodeSelector,
	}
}

func CreateDeckhouseDeploymentManifest(cfg *Config) *appsv1.Deployment {
	params := deckhouseDeploymentParamsFromCfg(cfg)

	return manifests.DeckhouseDeployment(params)
}

func WaitForKubernetesAPI(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for Kubernetes API to become Ready", 45, 5*time.Second).Run(func() error {
		_, err := kubeCl.Discovery().ServerVersion()
		if err == nil {
			return nil
		}
		return fmt.Errorf("kubernetes API is not Ready: %w", err)
	})
}

func PrepareDeckhouseInstallConfig(metaConfig *config.MetaConfig) (*Config, error) {
	clusterConfig, err := metaConfig.ClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal cluster config: %v", err)
	}

	providerClusterConfig, err := metaConfig.ProviderClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal provider config: %v", err)
	}

	staticClusterConfig, err := metaConfig.StaticClusterConfigYAML()
	if err != nil {
		return nil, fmt.Errorf("marshal static config: %v", err)
	}

	installConfig := Config{
		UUID:                  metaConfig.UUID,
		Registry:              metaConfig.Registry,
		DevBranch:             metaConfig.DeckhouseConfig.DevBranch,
		ReleaseChannel:        metaConfig.DeckhouseConfig.ReleaseChannel,
		Bundle:                metaConfig.DeckhouseConfig.Bundle,
		LogLevel:              metaConfig.DeckhouseConfig.LogLevel,
		DeckhouseConfig:       metaConfig.MergeDeckhouseConfig(),
		KubeDNSAddress:        metaConfig.ClusterDNSAddress,
		ProviderClusterConfig: providerClusterConfig,
		StaticClusterConfig:   staticClusterConfig,
		ClusterConfig:         clusterConfig,
	}

	return &installConfig, nil
}
