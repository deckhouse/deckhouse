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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

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
			name:     `Secret "d8-cluster-terraform-state"`,
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
					logboek.LogWarnLn("ERROR!")
					return err
				}
				logboek.LogInfoLn("OK!")
			}
		}
		return nil
	})
}

func WaitForReadiness(client *kube.KubernetesClient, cfg *Config) error {
	return logboek.LogProcess("Wait for deckhouse readiness", log.BoldOptions(), func() error {
		// watch for deckhouse pods in namespace become Ready

		ready := make(chan struct{}, 1)

		informer := kube.NewDeploymentInformer(client, context.Background())
		informer.Namespace = "d8-system"
		informer.FieldSelector = "metadata.name=deckhouse"
		//podInformer.LabelSelector = &metav1.LabelSelector{
		//	MatchLabels: nil,
		//	MatchExpressions: []metav1.LabelSelectorRequirement{
		//		{
		//			Key:      "app",
		//			Operator: "=",
		//			Values:   []string{"deckhouse"},
		//		},
		//	},
		//}
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
		go func() {
			for i := 1; i < 60; i++ {
				time.Sleep(time.Second)
				_ = PrintDeckhouseLogs(client, &stopLogsChan)
			}
		}()

		for {
			select {
			case <-checkTimer.C:
				continue
			case <-waitTimer.C:
				waitErr = fmt.Errorf("timeout while waiting for deckhouse deployment readiness. Check deckhouse queue and logs for errors")
			case <-ready:
				stopLogsChan <- struct{}{}
				logboek.LogInfoF("Deckhouse deployment is ready\n")
			}
			break
		}
		return waitErr
	})
}

func DeleteDeckhouseDeployment(client *kube.KubernetesClient) error {
	return logboek.LogProcess("Remove deckhouse", log.BoldOptions(), func() error {
		logboek.LogInfoF("Delete Deployment/deckhouse\n")
		err := client.AppsV1().Deployments("d8-system").Delete("deckhouse", &metav1.DeleteOptions{})
		if err != nil {
			logboek.LogWarnF("Error: %v\n", err)
		}

		return nil
	})
}

func CreateDeckhouseDeployment(client *kube.KubernetesClient, cfg *Config) error {
	image := cfg.Registry + ":" + strcase.ToKebab(cfg.ReleaseChannel)
	if cfg.ReleaseChannel == "" {
		image = cfg.Registry + "/dev:" + cfg.DevBranch
	}

	tasks := []createManifestTask{
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

	return logboek.LogProcess("Create Deployment", log.BoldOptions(), func() error {
		for _, task := range tasks {
			logboek.LogInfoF("Create %s\n", task.name)
			manifest := task.manifest()

			err := task.createTask(manifest)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
				logboek.LogWarnF("%s already exists. Trying to update ... ", task.name)
				err = task.updateTask(manifest)
				if err != nil {
					logboek.LogWarnLn("ERROR!")
					return err
				}
				logboek.LogInfoLn("OK!")
			}
		}
		return nil
	})
}

func CreateDeckhouseDeploymentManifest(cfg *Config) *appsv1.Deployment {
	image := cfg.Registry + ":" + strcase.ToKebab(cfg.ReleaseChannel)
	if cfg.ReleaseChannel == "" {
		image = cfg.Registry + "/dev:" + cfg.DevBranch
	}

	return generateDeckhouseDeployment(
		image, cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired(),
	)
}

func CreateNodeGroup(client *kube.KubernetesClient, data map[string]interface{}) error {
	return logboek.LogProcess("Create NodeGroup", log.BoldOptions(), func() error {
		doc := unstructured.Unstructured{}
		doc.SetUnstructuredContent(data)

		resourceSchema := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "nodegroups"}

		for i := 1; i < 45; i++ {
			res, err := client.Dynamic().Resource(resourceSchema).Create(&doc, metav1.CreateOptions{})
			if err == nil {
				logboek.LogInfoF("NodeGroup %q created\n", res.GetName())
				return nil
			}

			if errors.IsAlreadyExists(err) {
				logboek.LogWarnF("Object %v\n", err)
				return nil
			}

			logboek.LogInfoF("[Attempt #%v of 45] Waiting for NodeGroup to be created, next attempt in 15s\n", i)
			logboek.LogWarnF("%v\n\n", err)

			time.Sleep(15 * time.Second)
		}
		return fmt.Errorf("failed waiting for NodeGroup")
	})
}
