// Copyright 2026 Flant JSC
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
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
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
	err := retry.NewSilentLoop("get deployment", 30, 1*time.Second).RunContext(ctx, func() error {
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
		params, err := deckhouseDeploymentParamsFromCfg(cfg)
		if err != nil {
			return err
		}
		params.DeployTime = manifests.GetDeckhouseDeployTime(currentManifestInCluster)

		resDeployment = manifests.ParameterizeDeckhouseDeployment(currentManifestInCluster.DeepCopy(), params)

		return nil
	})

	return resDeployment, err
}

func controllerDeploymentTask(
	kubeCl *client.KubernetesClient,
	cfg *config.DeckhouseInstaller,
) (actions.ManifestTask, error) {
	// Build the manifest up front so the image-tag error surfaces here instead of inside the
	// Manifest callback (which has no way to return an error).
	deployment, err := CreateDeckhouseDeploymentManifest(cfg)
	if err != nil {
		return actions.ManifestTask{}, err
	}

	return actions.ManifestTask{
		Name: `Deployment "deckhouse"`,
		Manifest: func() any {
			return deployment
		},
		CreateFunc: func(ctx context.Context, manifest any) error {
			_, err := kubeCl.AppsV1().Deployments("d8-system").Create(ctx, manifest.(*appsv1.Deployment), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			preparedManifest, err := prepareDeckhouseDeploymentForUpdate(ctx, kubeCl, cfg, manifest.(*appsv1.Deployment))
			if err != nil {
				return err
			}

			_, err = kubeCl.AppsV1().Deployments("d8-system").Update(ctx, preparedManifest, metav1.UpdateOptions{})

			return err
		},
	}, nil
}

func LockDeckhouseQueueBeforeCreatingModuleConfigs(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
) (*actions.ManifestTask, error) {
	deckhouseDeploymentPresent := false

	err := retry.NewLoop("Get deckhouse manifest", 50, 1*time.Second).RunContext(ctx, func() error {
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
		Manifest: func() any {
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
		CreateFunc: func(ctx context.Context, manifest any) error {
			cm := manifest.(*apiv1.ConfigMap)

			_, err := kubeCl.
				CoreV1().ConfigMaps("d8-system").
				Create(ctx, cm, metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			return nil
		},
	}, nil
}

func UnlockDeckhouseQueueAfterCreatingModuleConfigs(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Unlock Deckhouse controller queue", 75, 1*time.Second).RunContext(ctx, func() error {
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
	namespaceTask := getNSTask(kubeCl)
	rbacTasks := getRBACTasks(kubeCl)
	installDataTasks := getInstallDataTasks(kubeCl, map[string]string{"version": cfg.InstallerVersion})
	registrySecretsTasks, err := getRegistryConfigTasks(ctx, kubeCl, cfg.Registry)
	if err != nil {
		return nil, err
	}
	tfStateTasks := getTFStateTasks(kubeCl, cfg)
	clusterConfigTasks := getClusterConfigTasks(kubeCl, cfg)
	clusterUUIDTasks := getClusterUUIDTasks(kubeCl, cfg)
	kubeDNSServiceTasks := getKubeDNSServiceTasks(kubeCl, cfg)

	prereqTasks := []actions.ManifestTask{}
	prereqTasks = append(prereqTasks, rbacTasks...)
	prereqTasks = append(prereqTasks, installDataTasks...)
	prereqTasks = append(prereqTasks, registrySecretsTasks...)
	prereqTasks = append(prereqTasks, tfStateTasks...)
	prereqTasks = append(prereqTasks, clusterConfigTasks...)
	prereqTasks = append(prereqTasks, clusterUUIDTasks...)

	if cfg.CommanderMode && cfg.CommanderUUID != uuid.Nil {
		prereqTasks = append(prereqTasks, commander.ConstructManagedByCommanderConfigMapTask(ctx, cfg.CommanderUUID, kubeCl))
	}

	prereqTasks = append(prereqTasks, kubeDNSServiceTasks...)

	if beforeDeckhouseTask != nil {
		err := beforeDeckhouseTask()
		if err != nil {
			return nil, err
		}
	}

	lockCmTask, err := LockDeckhouseQueueBeforeCreatingModuleConfigs(ctx, kubeCl)
	if err != nil {
		return nil, err
	}

	if lockCmTask != nil {
		prereqTasks = append(prereqTasks, *lockCmTask)
	}

	// The deckhouse controller Deployment is kept out of the prerequisite task
	// list and applied only after all of them exist. The pod and its hooks read
	// those resources on startup (registry pull secret, RBAC/ServiceAccount,
	// cluster-configuration secrets, d8-cluster-uuid, ...); racing the Deployment
	// against them can yield a pod that cannot pull images, or hooks that observe
	// half-initialized state — e.g. an empty global.discovery.clusterUUID, which
	// makes node-manager render node bootstrap scripts without the cluster-UUID
	// prefix for registry-packages-proxy and hangs CAPS-adopted nodes ~20m.
	deploymentTask, err := controllerDeploymentTask(kubeCl, cfg)
	if err != nil {
		return nil, err
	}

	result := &ManifestsResult{}

	// The first ModuleConfig's CRD is installed by the now-running deckhouse
	// pod, so it retry-waits for the CRD to appear; the rest can only succeed
	// once that CRD exists, so they run after it, not alongside it.
	var moduleConfigCRDTask *actions.ManifestTask
	var moduleConfigTasks []actions.ManifestTask
	if len(cfg.ModuleConfigs) > 0 {
		prepareModuleConfig(ctx, cfg.ModuleConfigs[0], result)
		crdTask := createModuleConfigManifestTask(kubeCl, cfg.ModuleConfigs[0], "Waiting for creating ModuleConfig CRD...")
		moduleConfigCRDTask = &crdTask

		for i := 1; i < len(cfg.ModuleConfigs); i++ {
			prepareModuleConfig(ctx, cfg.ModuleConfigs[i], result)
			moduleConfigTasks = append(moduleConfigTasks, createModuleConfigManifestTask(kubeCl, cfg.ModuleConfigs[i], ""))
		}
	}

	err = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create Manifests", func(ctx context.Context) error {
		// Tasks use Get-then-Create or simple Create-or-Update flows and target
		// distinct API resources. Retry interval is 1s (was 5s): the loop reacts to
		// a transient error clearing (e.g. the ModuleConfig CRD appearing) in ~1s
		// instead of dead-waiting; the total deadline stays 600 attempts × 1s.
		runTask := func(task actions.ManifestTask) error {
			return retry.NewSilentLoop(task.Name, 600, 1*time.Second).RunContext(
				ctx,
				func() error {
					return task.CreateOrUpdate(ctx)
				},
			)
		}

		// runParallel applies independent tasks concurrently, capping the number of
		// parallel writes so we don't hammer the freshly bootstrapped apiserver.
		runParallel := func(parallelTasks []actions.ManifestTask) error {
			const maxParallel = 8
			eg, egCtx := errgroup.WithContext(ctx)
			eg.SetLimit(maxParallel)

			var logMu sync.Mutex
			for i := range parallelTasks {
				task := parallelTasks[i]
				eg.Go(func() error {
					if egCtx.Err() != nil {
						return egCtx.Err()
					}
					if err := runTask(task); err != nil {
						logMu.Lock()
						dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("manifest task %q failed: %v", task.Name, err))
						logMu.Unlock()
						return err
					}
					return nil
				})
			}

			return eg.Wait()
		}

		// The d8-system Namespace comes first; everything else lives in it or
		// references it.
		if err := runTask(namespaceTask); err != nil {
			return err
		}

		// All deckhouse prerequisites (RBAC, ServiceAccount, registry pull
		// secret, cluster-configuration secrets, d8-cluster-uuid, queue lock,
		// ...) are independent of each other and applied concurrently — but
		// before the controller Deployment below, which reads them on startup.
		if err := runParallel(prereqTasks); err != nil {
			return err
		}

		// The controller Deployment, once all its prerequisites exist.
		if err := runTask(deploymentTask); err != nil {
			return err
		}

		if moduleConfigCRDTask != nil {
			// This one waits (via retry) for the CRD the now-running deckhouse pod
			// installs; the rest depend on that CRD existing, so it must land first.
			if err := runTask(*moduleConfigCRDTask); err != nil {
				return err
			}
		}

		// Remaining ModuleConfigs are independent of each other.
		return runParallel(moduleConfigTasks)
	})
	if err != nil {
		return nil, err
	}

	return result, UnlockDeckhouseQueueAfterCreatingModuleConfigs(ctx, kubeCl)
}

func WaitForReadiness(ctx context.Context, kubeCl *client.KubernetesClient, timeout time.Duration) error {
	return WaitForReadinessNotOnNode(ctx, kubeCl, "", timeout)
}

func WaitForReadinessNotOnNode(ctx context.Context, kubeCl *client.KubernetesClient, excludeNode string, timeout time.Duration) error {
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Waiting for Deckhouse to become Ready", func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
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
					dhlog.FromContext(ctx).InfoContext(ctx, err.Error())
				}

				if ok {
					dhlog.FromContext(ctx).InfoContext(ctx, "Deckhouse pod is Ready!\n")
					return nil
				}

				time.Sleep(1 * time.Second)
			}
		}
	})
}

func CreateDeckhouseDeployment(ctx context.Context, kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) error {
	task, err := controllerDeploymentTask(kubeCl, cfg)
	if err != nil {
		return err
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create Deployment", task.CreateOrUpdate)
}

func deckhouseDeploymentParamsFromCfg(cfg *config.DeckhouseInstaller) (manifests.DeckhouseDeploymentParams, error) {
	image, err := cfg.GetInclusterImage(context.Background(), true)
	if err != nil {
		return manifests.DeckhouseDeploymentParams{}, err
	}

	return manifests.DeckhouseDeploymentParams{
		Registry:           image,
		LogLevel:           cfg.LogLevel,
		Bundle:             cfg.Bundle,
		KubeadmBootstrap:   cfg.KubeadmBootstrap,
		MasterNodeSelector: cfg.MasterNodeSelector,
	}, nil
}

func CreateDeckhouseDeploymentManifest(cfg *config.DeckhouseInstaller) (*appsv1.Deployment, error) {
	params, err := deckhouseDeploymentParamsFromCfg(cfg)
	if err != nil {
		return nil, err
	}

	return manifests.DeckhouseDeployment(params), nil
}

func WaitForKubernetesAPI(ctx context.Context, kubeCl *client.KubernetesClient) error {
	return retry.NewLoop("Waiting for Kubernetes API to become Ready", 225, 1*time.Second).
		RunContext(ctx, func() error {
			_, err := kubeCl.Discovery().ServerVersion()
			if err == nil {
				return nil
			}
			return fmt.Errorf("kubernetes API is not Ready: %w", err)
		})
}

// helpers to get tasks
func getNSTask(kubeCl *client.KubernetesClient) actions.ManifestTask {
	return actions.ManifestTask{
		Name: `Namespace "d8-system"`,
		Manifest: func() any {
			return manifests.DeckhouseNamespace("d8-system")
		},
		CreateFunc: func(ctx context.Context, manifest any) error {
			_, err := kubeCl.
				CoreV1().Namespaces().
				Create(ctx, manifest.(*apiv1.Namespace), metav1.CreateOptions{})
			return err
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			_, err := kubeCl.
				CoreV1().
				Namespaces().
				Update(ctx, manifest.(*apiv1.Namespace), metav1.UpdateOptions{})
			return err
		},
	}
}

func getRBACTasks(kubeCl *client.KubernetesClient) []actions.ManifestTask {
	return []actions.ManifestTask{
		{
			Name:     `Admin ClusterRole "cluster-admin"`,
			Manifest: func() any { return manifests.DeckhouseAdminClusterRole() },
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					RbacV1().ClusterRoles().
					Get(ctx, manifest.(*rbacv1.ClusterRole).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.RbacV1().ClusterRoles().Create(ctx, manifest.(*rbacv1.ClusterRole), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					RbacV1().ClusterRoles().
					Update(ctx, manifest.(*rbacv1.ClusterRole), metav1.UpdateOptions{})

				return err
			},
		},
		{
			Name:     `ClusterRoleBinding "deckhouse"`,
			Manifest: func() any { return manifests.DeckhouseAdminClusterRoleBinding() },
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					RbacV1().ClusterRoleBindings().
					Get(ctx, manifest.(*rbacv1.ClusterRoleBinding).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.RbacV1().ClusterRoleBindings().Create(ctx, manifest.(*rbacv1.ClusterRoleBinding), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					RbacV1().ClusterRoleBindings().
					Update(ctx, manifest.(*rbacv1.ClusterRoleBinding), metav1.UpdateOptions{})

				return err
			},
		},
		{
			Name:     `ServiceAccount "deckhouse"`,
			Manifest: func() any { return manifests.DeckhouseServiceAccount() },
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().ServiceAccounts("d8-system").
					Get(ctx, manifest.(*apiv1.ServiceAccount).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.CoreV1().ServiceAccounts("d8-system").Create(ctx, manifest.(*apiv1.ServiceAccount), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().ServiceAccounts("d8-system").
					Update(ctx, manifest.(*apiv1.ServiceAccount), metav1.UpdateOptions{})

				return err
			},
		},
	}
}

func getInstallDataTasks(kubeCl *client.KubernetesClient, data map[string]string) []actions.ManifestTask {
	return []actions.ManifestTask{
		{
			Name: `ConfigMap "install-data"`,
			Manifest: func() any {
				return &apiv1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "install-data",
						Namespace: "d8-system",
					},
					Data: data,
				}
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				cm := manifest.(*apiv1.ConfigMap)

				_, err := kubeCl.
					CoreV1().ConfigMaps("d8-system").
					Create(ctx, cm, metav1.CreateOptions{})

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				cm := manifest.(*apiv1.ConfigMap)

				_, err := kubeCl.
					CoreV1().ConfigMaps("d8-system").
					Update(ctx, cm, metav1.UpdateOptions{})

				return err
			},
		},
	}
}

func getRegistryConfigTasks(ctx context.Context, kubeCl *client.KubernetesClient, cfg registry.Config) ([]actions.ManifestTask, error) {
	tasks := []actions.ManifestTask{}
	deckhouseRegistrySecretData, err := cfg.
		Manifest().
		DeckhouseRegistrySecretData(
			func() (registry.PKI, error) {
				return registry.GetPKI(ctx, kubeCl)
			},
		)
	if err != nil {
		return nil, fmt.Errorf("create deckhouse registry secret data: %w", err)
	}

	tasks = append(tasks, actions.ManifestTask{
		Name: `Secret "deckhouse-registry"`,
		Manifest: func() any {
			return manifests.DeckhouseRegistrySecret(deckhouseRegistrySecretData)
		},
		CreateFunc: func(ctx context.Context, manifest any) error {
			_, err = kubeCl.
				CoreV1().Secrets("d8-system").
				Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})

			if err != nil && apierrors.IsAlreadyExists(err) {
				dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				return nil
			}
			return err
		},
		UpdateFunc: func(ctx context.Context, manifest any) error {
			_, err := kubeCl.
				CoreV1().Secrets("d8-system").
				Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})

			return err
		},
	})

	isExist, registryBashibleConfigSecretData, err := cfg.
		Manifest().
		RegistryBashibleConfigSecretData(
			func() (registry.PKI, error) {
				return registry.GetPKI(ctx, kubeCl)
			},
		)
	if err != nil {
		return nil, fmt.Errorf("create registry bashible config secret data: %w", err)
	}

	if isExist {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Secret "registry-bashible-config"`,
			Manifest: func() any {
				return manifests.RegistryBashibleConfigSecret(registryBashibleConfigSecretData)
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err = kubeCl.
					CoreV1().Secrets("d8-system").
					Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})

				if err != nil && apierrors.IsAlreadyExists(err) {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
					return nil
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("d8-system").
					Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})

				return err
			},
		})
	}

	return tasks, nil
}

func getTFStateTasks(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) []actions.ManifestTask {
	tasks := []actions.ManifestTask{}
	if len(cfg.InfrastructureState) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-terraform-state"`,
			Manifest: func() any { return manifests.SecretWithInfrastructureState(cfg.InfrastructureState) },
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("d8-system").
					Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Secrets("d8-system").
							Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("d8-system").
					Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})

				return err
			},
		})
	}

	for nodeName, tfState := range cfg.NodesInfrastructureState {
		getManifest := func() any {
			return manifests.SecretWithNodeInfrastructureState(nodeName, "master", tfState, nil)
		}
		tasks = append(tasks, actions.ManifestTask{
			Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			Manifest: getManifest,
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("d8-system").
					Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})

				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Secrets("d8-system").
							Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("d8-system").
					Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})

				return err
			},
		})
	}

	return tasks
}

func getClusterConfigTasks(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) []actions.ManifestTask {
	tasks := []actions.ManifestTask{}
	if len(cfg.ClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name:     `Secret "d8-cluster-configuration"`,
			Manifest: func() any { return manifests.SecretWithClusterConfig(cfg.ClusterConfig) },
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("kube-system").
					Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Secrets("kube-system").
							Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("kube-system").
					Update(ctx, manifest.(*apiv1.Secret), metav1.UpdateOptions{})

				return err
			},
		})
	}

	if len(cfg.ProviderClusterConfig) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Secret "d8-provider-cluster-configuration"`,
			Manifest: func() any {
				return manifests.SecretWithProviderClusterConfig(
					cfg.ProviderClusterConfig, cfg.CloudDiscovery,
				)
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("kube-system").
					Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Secrets("kube-system").
							Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}

				_, err = kubeCl.
					CoreV1().Secrets("kube-system").
					Patch(
						ctx,
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
			Manifest: func() any {
				return manifests.SecretWithStaticClusterConfig(cfg.StaticClusterConfig)
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Secrets("kube-system").
					Get(ctx, manifest.(*apiv1.Secret).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Secrets("kube-system").
							Create(ctx, manifest.(*apiv1.Secret), metav1.CreateOptions{})
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				data, err := json.Marshal(manifest.(*apiv1.Secret))
				if err != nil {
					return err
				}

				_, err = kubeCl.
					CoreV1().Secrets("kube-system").
					Patch(
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

	return tasks
}

func getClusterUUIDTasks(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) []actions.ManifestTask {
	tasks := []actions.ManifestTask{}
	if len(cfg.UUID) > 0 {
		tasks = append(tasks, actions.ManifestTask{
			Name: `ConfigMap "d8-cluster-uuid"`,
			Manifest: func() any {
				return manifests.ClusterUUIDConfigMap(cfg.UUID)
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).
					Create(ctx, manifest.(*apiv1.ConfigMap), metav1.CreateOptions{})

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().ConfigMaps(manifests.ClusterUUIDCmNamespace).
					Update(ctx, manifest.(*apiv1.ConfigMap), metav1.UpdateOptions{})

				return err
			},
		})
	}

	return tasks
}

func getKubeDNSServiceTasks(kubeCl *client.KubernetesClient, cfg *config.DeckhouseInstaller) []actions.ManifestTask {
	tasks := []actions.ManifestTask{}
	if cfg.KubeDNSAddress != "" {
		tasks = append(tasks, actions.ManifestTask{
			Name: `Service "kube-dns"`,
			Manifest: func() any {
				return manifests.KubeDNSService(cfg.KubeDNSAddress)
			},
			CreateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Services("kube-system").
					Get(ctx, manifest.(*apiv1.Service).GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						_, err = kubeCl.
							CoreV1().Services("kube-system").
							Create(ctx, manifest.(*apiv1.Service), metav1.CreateOptions{})
						if err != nil && strings.Contains(err.Error(), "provided IP is already allocated") {
							dhlog.FromContext(ctx).InfoContext(ctx, "Service for DNS already exists. Skip!")
							return nil
						}
					}
				} else {
					dhlog.FromContext(ctx).InfoContext(ctx, "Already exists. Skip!")
				}

				return err
			},
			UpdateFunc: func(ctx context.Context, manifest any) error {
				_, err := kubeCl.
					CoreV1().Services("kube-system").
					Update(ctx, manifest.(*apiv1.Service), metav1.UpdateOptions{})

				return err
			},
		})
	}

	return tasks
}
