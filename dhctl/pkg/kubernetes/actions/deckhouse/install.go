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

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func prepareDeckhouseDeploymentForUpdate(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.DeckhouseInstaller,
	manifestForUpdate *appsv1.Deployment,
) (*appsv1.Deployment, error) {
	resDeployment := manifestForUpdate
	err := retry.NewSilentLoop("get deployment", 10, 3*time.Second).RunContext(ctx, func() error {
		currentManifestInCluster, err := kubeCl.AppsV1().
			Deployments(manifestForUpdate.GetNamespace()).
			Get(ctx, manifestForUpdate.GetName(), metav1.GetOptions{})
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

func controllerDeploymentTask(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.DeckhouseInstaller,
) actions.ManifestTask {
	return actions.ManifestTask{
		Name: `Deployment "deckhouse"`,
		Manifest: func() interface{} {
			return CreateDeckhouseDeploymentManifest(cfg)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Create(ctx, manifest.(*appsv1.Deployment), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			preparedManifest, err := prepareDeckhouseDeploymentForUpdate(ctx, kubeCl, cfg, manifest.(*appsv1.Deployment))
			if err != nil {
				return err
			}

			_, err = kubeCl.AppsV1().Deployments("d8-system").Update(ctx, preparedManifest, metav1.UpdateOptions{})

			return err
		},
	}
}

func LockDeckhouseQueueBeforeCreatingModuleConfigs(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) (*actions.ManifestTask, error) {
	deckhouseDeploymentPresent := false

	err := retry.NewLoop("Get deckhouse manifest", 10, 5*time.Second).RunContext(ctx, func() error {
		_, err := kubeCl.AppsV1().Deployments("d8-system").Get(ctx, "deckhouse", metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}

			deckhouseDeploymentPresent = false
			return nil
		}

		deckhouseDeploymentPresent = true
		return nil
	})

	if err != nil {
		return nil, err
	}

	if deckhouseDeploymentPresent {
		// we need create lock cm only one first deckhouse install attempt
		return nil, nil
	}

	return &actions.ManifestTask{
		Name: `ConfigMap "deckhouse-bootstrap-lock"`,
		Manifest: func() interface{} {
			return &apiv1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deckhouse-bootstrap-lock",
					Namespace: "d8-system",
				},
			}
		},
		CreateFunc: func(manifest interface{}) error {
			cm := manifest.(*apiv1.ConfigMap)
			_, err := kubeCl.CoreV1().ConfigMaps("d8-system").
				Create(ctx, cm, metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			return nil
		},
	}, nil
}

func UnlockDeckhouseQueueAfterCreatingModuleConfigs(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Unlock Deckhouse controller queue", 15, 5*time.Second).RunContext(ctx, func() error {
		err := kubeCl.CoreV1().ConfigMaps("d8-system").
			Delete(ctx, "deckhouse-bootstrap-lock", metav1.DeleteOptions{})

		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	})
}

type ManifestsResult struct {
	WithResourcesMCTasks []actions.ModuleConfigTask
	PostBootstrapMCTasks []actions.ModuleConfigTask
}

func CreateDeckhouseManifests(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	cfg *config.DeckhouseInstaller,
	beforeDeckhouseTask func() error,
) (*ManifestsResult, error) {
	tasks := []actions.ManifestTask{
		{
			Name:     `Namespace "d8-system"`,
			Manifest: func() interface{} { return manifests.DeckhouseNamespace("d8-system") },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Namespaces().Get(ctx, manifest.(*apiv1.Namespace).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Namespaces().Create(ctx, manifest.(*apiv1.Namespace), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				return nil
			},
		},
		{
			Name:     `Admin ClusterRole "cluster-admin"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRole() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Get(ctx, manifest.(*rbacv1.ClusterRole).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.RbacV1().ClusterRoles().Create(ctx, manifest.(*rbacv1.ClusterRole), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoles().Update(ctx, manifest.(*rbacv1.ClusterRole), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `ClusterRoleBinding "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseAdminClusterRoleBinding() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Get(ctx, manifest.(*rbacv1.ClusterRoleBinding).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, manifest.(*rbacv1.ClusterRoleBinding), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.RbacV1().ClusterRoleBindings().Update(ctx, manifest.(*rbacv1.ClusterRoleBinding), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name:     `ServiceAccount "deckhouse"`,
			Manifest: func() interface{} { return manifests.DeckhouseServiceAccount() },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Get(ctx, manifest.(*apiv1.ServiceAccount).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().ServiceAccounts("d8-system").Create(ctx, manifest.(*apiv1.ServiceAccount), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ServiceAccounts("d8-system").Update(ctx, manifest.(*apiv1.ServiceAccount), metav1.UpdateOptions{})
				return err
			},
		},
		{
			Name: `ConfigMap "install-data"`,
			Manifest: func() interface{} {
				return &apiv1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "install-data",
						Namespace: "d8-system",
					},
					Data: map[string]string{
						"version": cfg.InstallerVersion,
					},
				}
			},
			CreateFunc: func(manifest interface{}) error {
				cm := manifest.(*apiv1.ConfigMap)
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").
					Create(ctx, cm, metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				cm := manifest.(*apiv1.ConfigMap)
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").
					Update(ctx, cm, metav1.UpdateOptions{})
				return err
			},
		},
	}

	registryBulder := cfg.Registry.
		Builder().
		WithPKI(registry.NewPKIK8SProvider())
	// Registry secrets
	deckhouseRegistrySecretData, err := registryBulder.DeckhouseRegistrySecretData()
	if err != nil {
		return nil, err
	}
	registryInitSecretData, err := registryBulder.RegistryInitSecretData()
	if err != nil {
		return nil, err
	}
	registryBashibleConfigSecretData, err := registryBulder.RegistryBashibleConfigSecretData()
	if err != nil {
		return nil, err
	}
	tasks = append(tasks, actions.ManifestTask{
		Name:     `Secret "deckhouse-registry"`,
		Manifest: func() interface{} { return manifests.DeckhouseRegistrySecret(deckhouseRegistrySecretData) },
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	})
	tasks = append(tasks, actions.ManifestTask{
		Name:     `Secret "registry-init"`,
		Manifest: func() interface{} { return manifests.RegistryInitSecret(registryInitSecretData) },
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	})
	tasks = append(tasks, actions.ManifestTask{
		Name: `Secret "registry-bashible-config"`,
		Manifest: func() interface{} {
			return manifests.RegistryBashibleConfigSecret(registryBashibleConfigSecretData)
		},
		CreateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
			return err
		},
	})

	if len(cfg.InfrastructureState) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-terraform-state"`,
			Manifest: func() interface{} { return manifests.SecretWithInfrastructureState(cfg.InfrastructureState) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		})
	}

	for nodeName, tfState := range cfg.NodesInfrastructureState {
		getManifest := func() interface{} {
			return manifests.SecretWithNodeInfrastructureState(nodeName, "master", tfState, nil)
		}
		tasks = append(tasks, actions.ManifestTask{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getManifest,
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Secrets("d8-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
				return err
			},
		})
	}

	if len(cfg.ClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() interface{} { return manifests.SecretWithClusterConfig(cfg.ClusterConfig) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("kube-system").Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})
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
				_, err := kubeCl.CoreV1().Secrets("kube-system").Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(ctx,
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
				_, err := kubeCl.CoreV1().Secrets("kube-system").Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Secrets("kube-system").Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}
				_, err = kubeCl.CoreV1().Secrets("kube-system").Patch(
					ctx,
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
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(ctx, manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Update(ctx, manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
				return err
			},
		})
	}

	if cfg.CommanderMode && cfg.CommanderUUID != uuid.Nil {
		tasks = append(tasks, commander.ConstructManagedByCommanderConfigMapTask(ctx, cfg.CommanderUUID, kubeCl))
	}

	if cfg.KubeDNSAddress != "" {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Service "kube-dns"`,
			Manifest: func() interface{} {
				return manifests.KubeDNSService(cfg.KubeDNSAddress)
			},
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Services("kube-system").Get(ctx, manifest.(*apiv1.Service).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().Services("kube-system").Create(ctx, manifest.(*apiv1.Service), metav1.CreateOptions{})
						if err != nil && strings.Contains(err.Error(), "provided IP is already allocated") {
							log.InfoLn("Service for DNS already exists. Skip!")
							return nil
						}
					}
				} else {
					log.InfoLn("Already exists. Skip!")
				}
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Services("kube-system").Update(ctx, manifest.(*apiv1.Service), metav1.UpdateOptions{})
				return err
			},
		})
	}

	err = beforeDeckhouseTask()
	if err != nil {
		return nil, err
	}

	lockCmTask, err := LockDeckhouseQueueBeforeCreatingModuleConfigs(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	if lockCmTask != nil {
		tasks = append(tasks, *lockCmTask)
	}

	tasks = append(tasks, controllerDeploymentTask(ctx, kubeCl, cfg))

	result := &ManifestsResult{}

	if len(cfg.ModuleConfigs) > 0 {
		prepareModuleConfig(ctx, cfg.ModuleConfigs[0], result)
		tasks = append(tasks, createModuleConfigManifestTask(ctx, kubeCl, cfg.ModuleConfigs[0], "Waiting for creating ModuleConfig CRD..."))

		for i := 1; i < len(cfg.ModuleConfigs); i++ {
			prepareModuleConfig(ctx, cfg.ModuleConfigs[i], result)
			tasks = append(tasks, createModuleConfigManifestTask(ctx, kubeCl, cfg.ModuleConfigs[i], ""))
		}
	}

	err = log.Process("default", "Create Manifests", func() error {
		for _, task := range tasks {
			err := retry.NewSilentLoop(task.Name, 60, 5*time.Second).RunContext(ctx, task.CreateOrUpdate)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, UnlockDeckhouseQueueAfterCreatingModuleConfigs(ctx, kubeCl)
}

func WaitForReadiness(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return WaitForReadinessNotOnNode(ctx, kubeCl, "")
}

func WaitForReadinessNotOnNode(ctx context.Context, kubeCl *client.KubernetesClient, excludeNode string) error {
	return log.Process("default", "Waiting for Deckhouse to become Ready", func() error {
		ctx, cancel := context.WithTimeout(ctx, app.DeckhouseTimeout)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return ErrTimedOut
			default:
				ok, err := NewLogPrinter(kubeCl).
					WithLeaderElectionAwarenessMode(types.NamespacedName{Namespace: "d8-system", Name: "deckhouse-leader-election"}).
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
					log.Success("Deckhouse pod is Ready!\n")
					return nil
				}

				time.Sleep(5 * time.Second)
			}
		}
	})
}

func CreateDeckhouseDeployment(ctx context.Context, kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
	task := controllerDeploymentTask(ctx, kubeCl, cfg)

	return log.Process("default", "Create Deployment", task.CreateOrUpdate)
}

func deckhouseDeploymentParamsFromCfg(cfg *config.DeckhouseInstaller) manifests.DeckhouseDeploymentParams {
	return manifests.DeckhouseDeploymentParams{
		Registry:           cfg.GetImage(true),
		LogLevel:           cfg.LogLevel,
		Bundle:             cfg.Bundle,
		KubeadmBootstrap:   cfg.KubeadmBootstrap,
		MasterNodeSelector: cfg.MasterNodeSelector,
	}
}

func CreateDeckhouseDeploymentManifest(cfg *config.DeckhouseInstaller) *appsv1.Deployment {
	params := deckhouseDeploymentParamsFromCfg(cfg)

	return manifests.DeckhouseDeployment(params)
}

func WaitForKubernetesAPI(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for Kubernetes API to become Ready", 45, 5*time.Second).
		RunContext(ctx, func() error {
			_, err := kubeCl.Discovery().ServerVersion()
			if err == nil {
				return nil
			}
			return fmt.Errorf("kubernetes API is not Ready: %w", err)
		})
}
