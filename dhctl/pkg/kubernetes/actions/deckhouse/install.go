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
	"os"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func prepareDeckhouseDeploymentForUpdate(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller, manifestForUpdate *appsv1.Deployment) (*appsv1.Deployment, error) {
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

func controllerDeploymentTask(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) actions.ManifestTask {
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

func LockDeckhouseQueueBeforeCreatingModuleConfigs(kubeCl *client.KubernetesClient) (*actions.ManifestTask, error) {
	deckhouseDeploymentPresent := false

	err := retry.NewLoop("Get deckhouse manifest", 10, 5*time.Second).Run(func() error {
		_, err := kubeCl.AppsV1().Deployments("d8-system").Get(context.TODO(), "deckhouse", metav1.GetOptions{})
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
				Create(context.TODO(), cm, metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(manifest interface{}) error {
			return nil
		},
	}, nil
}

func UnlockDeckhouseQueueAfterCreatingModuleConfigs(kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Unlock Deckhouse controller queue", 15, 5*time.Second).Run(func() error {
		err := kubeCl.CoreV1().ConfigMaps("d8-system").
			Delete(context.TODO(), "deckhouse-bootstrap-lock", metav1.DeleteOptions{})

		if apierrors.IsNotFound(err) {
			return nil
		}

		return err
	})
}

func ConfigureReleaseChannel(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
	// if we have correct semver version we should create Deckhouse Release for prevent rollback on previous version
	// if installer version > version in release channel
	if tag, found := config.ReadVersionTagFromInstallerContainer(); found {
		deckhouseRelease := unstructured.Unstructured{}
		deckhouseRelease.SetUnstructuredContent(map[string]interface{}{
			"apiVersion": "deckhouse.io/v1alpha1",
			"kind":       "DeckhouseRelease",
			"metadata": map[string]interface{}{
				"name": tag,
			},
			"spec": map[string]interface{}{
				"version": tag,
			},
		})

		err := retry.NewLoop(fmt.Sprintf("Create deckhouse release for version %s", tag), 15, 5*time.Second).
			BreakIf(apierrors.IsAlreadyExists).
			Run(func() error {
				_, err := kubeCl.Dynamic().Resource(v1alpha1.DeckhouseReleaseGVR).Create(context.TODO(), &deckhouseRelease, metav1.CreateOptions{})
				if err != nil {
					return err
				}

				return nil
			})
		if err != nil {
			return err
		}
	}

	if cfg.ReleaseChannel == "" {
		return nil
	}
	// save release channel into module config we do not set it in Deckhouse mc because we want to install deckhouse only one release
	return retry.NewLoop("Set release channel to deckhouse module config", 15, 5*time.Second).
		BreakIf(apierrors.IsNotFound).
		Run(func() error {
			cm, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), "deckhouse", metav1.GetOptions{})
			if err != nil {
				return err
			}

			err = unstructured.SetNestedField(cm.Object, cfg.ReleaseChannel, "spec", "settings", "releaseChannel")
			if err != nil {
				return err
			}

			_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Update(context.TODO(), cm, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			return nil
		})
}

func CreateDeckhouseManifests(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
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
					Create(context.TODO(), cm, metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				cm := manifest.(*apiv1.ConfigMap)
				_, err := kubeCl.CoreV1().ConfigMaps("d8-system").
					Update(context.TODO(), cm, metav1.UpdateOptions{})
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
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Create(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).Update(context.TODO(), manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})
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

	lockCmTask, err := LockDeckhouseQueueBeforeCreatingModuleConfigs(kubeCl)
	if err != nil {
		return err
	}
	if lockCmTask != nil {
		tasks = append(tasks, *lockCmTask)
	}

	tasks = append(tasks, controllerDeploymentTask(kubeCl, cfg))

	if len(cfg.ModuleConfigs) > 0 {
		createTask := func(mc *config.ModuleConfig, createMsg string) actions.ManifestTask {
			mcUnstructMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(mc)
			if err != nil {
				panic(err)
			}
			mcUnstruct := &unstructured.Unstructured{Object: mcUnstructMap}
			return actions.ManifestTask{
				Name: fmt.Sprintf(`ModuleConfig "%s"`, mc.GetName()),
				Manifest: func() interface{} {
					return mcUnstruct
				},
				CreateFunc: func(manifest interface{}) error {
					if createMsg != "" {
						log.InfoLn(createMsg)
					}
					// fake client does not support cache
					if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
						// need for invalidate cache
						_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
						if err != nil {
							log.DebugF("Error getting mc api resource: %v\n", err)
						}
					}

					_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).
						Create(context.TODO(), manifest.(*unstructured.Unstructured), metav1.CreateOptions{})
					if err != nil {
						log.InfoF("Do not create mc: %v\n", err)
					}

					return err
				},
				UpdateFunc: func(manifest interface{}) error {
					// fake client does not support cache
					if _, ok := os.LookupEnv("DHCTL_TEST"); !ok {
						// need for invalidate cache
						_, err := kubeCl.APIResource(config.ModuleConfigGroup+"/"+config.ModuleConfigVersion, config.ModuleConfigKind)
						if err != nil {
							log.DebugF("Error getting mc api resource: %v\n", err)
						}
					}

					newManifest := manifest.(*unstructured.Unstructured)

					oldManifest, err := kubeCl.Dynamic().Resource(config.ModuleConfigGVR).Get(context.TODO(), newManifest.GetName(), metav1.GetOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						log.DebugF("Error getting mc: %v\n", err)
					} else {
						newManifest.SetResourceVersion(oldManifest.GetResourceVersion())
					}

					_, err = kubeCl.Dynamic().Resource(config.ModuleConfigGVR).
						Update(context.TODO(), newManifest, metav1.UpdateOptions{})
					if err != nil {
						log.InfoF("Do not updating mc: %v\n", err)
					}

					return err
				},
			}
		}

		tasks = append(tasks, createTask(cfg.ModuleConfigs[0], "Waiting for creating ModuleConfig CRD..."))

		for i := 1; i < len(cfg.ModuleConfigs); i++ {
			tasks = append(tasks, createTask(cfg.ModuleConfigs[i], ""))
		}
	}

	err = log.Process("default", "Create Manifests", func() error {
		for _, task := range tasks {
			err := retry.NewSilentLoop(task.Name, 60, 5*time.Second).Run(task.CreateOrUpdate)
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return UnlockDeckhouseQueueAfterCreatingModuleConfigs(kubeCl)
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

func CreateDeckhouseDeployment(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
	task := controllerDeploymentTask(kubeCl, cfg)

	return log.Process("default", "Create Deployment", task.CreateOrUpdate)
}

func deckhouseDeploymentParamsFromCfg(cfg *config.DeckhouseInstaller) manifests.DeckhouseDeploymentParams {
	return manifests.DeckhouseDeploymentParams{
		Registry:           cfg.GetImage(true),
		LogLevel:           cfg.LogLevel,
		Bundle:             cfg.Bundle,
		IsSecureRegistry:   cfg.IsRegistryAccessRequired(),
		KubeadmBootstrap:   cfg.KubeadmBootstrap,
		MasterNodeSelector: cfg.MasterNodeSelector,
	}
}

func CreateDeckhouseDeploymentManifest(cfg *config.DeckhouseInstaller) *appsv1.Deployment {
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
