package deckhouse

import (
	"context"
	"encoding/base64"
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

	"flant/deckhouse-candi/pkg/config"
	"flant/deckhouse-candi/pkg/kube"
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

type createManifestTask struct {
	name       string
	createTask func(manifest interface{}) error
	updateTask func(manifest interface{}) error
	manifest   func() interface{}
}

func CreateDeckhouseManifests(client *kube.KubernetesClient, cfg *Config) error {
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
				_, err := client.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	}

	for nodeName, tfState := range cfg.NodesTerraformState {
		getManifest := func() interface{} { return generateSecretWithNodeTerraformState(nodeName, "master", tfState) }
		tasks = append(tasks, createManifestTask{
			name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			manifest: getManifest,
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

	tasks = append(tasks, createManifestTask{
		name: `Deployment "deckhouse"`,
		manifest: func() interface{} {
			return generateDeckhouseDeployment(
				cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired(),
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
	})

	return logboek.LogProcess("Create Manifests", log.BoldOptions(), func() error {
		for _, task := range tasks {
			err := runTask(task)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func runTask(task createManifestTask) error {
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
	return nil
}

func SaveNodeTerraformState(client *kube.KubernetesClient, nodeName, nodeGroup string, tfState []byte) error {
	getManifest := func() interface{} { return generateSecretWithNodeTerraformState(nodeName, nodeGroup, tfState) }
	return retry.StartLoop(fmt.Sprintf("Save Terraform state for Node %q", nodeName), 45, 10, func() error {
		return runTask(createManifestTask{
			name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
			manifest: getManifest,
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	})
}

func DeleteTerraformState(client *kube.KubernetesClient, secretName string) error {
	return retry.StartLoop(fmt.Sprintf("Save Terraform %q", secretName), 45, 10, func() error {
		return client.CoreV1().Secrets("d8-system").Delete(secretName, &metav1.DeleteOptions{})
	})
}

func SaveClusterTerraformState(client *kube.KubernetesClient, tfState []byte) error {
	return retry.StartLoop("Save Cluster Terraform state", 45, 10, func() error {
		return runTask(createManifestTask{
			name:     `Secret "d8-cluster-terraform-state"`,
			manifest: func() interface{} { return generateSecretWithTerraformState(tfState) },
			createTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			updateTask: func(manifest interface{}) error {
				_, err := client.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		})
	})
}

func WaitForReadiness(client *kube.KubernetesClient, cfg *Config) error {
	return logboek.LogProcess("Waiting for Deckhouse readiness", log.BoldOptions(), func() error {
		// watch for deckhouse pods in namespace become Ready
		ready := make(chan struct{}, 1)

		informer := kube.NewDeploymentInformer(context.Background(), client)
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
				err = PrintDeckhouseLogs(client, &stopLogsChan)
				if err != nil {
					logboek.LogInfoF("Deckhouse is not ready yet - %v\n", err)
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
	tasks := []createManifestTask{
		{
			name: `Deployment "deckhouse"`,
			manifest: func() interface{} {
				return generateDeckhouseDeployment(cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired())
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
	return generateDeckhouseDeployment(cfg.GetImage(), cfg.LogLevel, cfg.Bundle, cfg.IsRegistryAccessRequired())
}

func CreateNodeGroup(client *kube.KubernetesClient, nodeGroupName string, data map[string]interface{}) error {
	doc := unstructured.Unstructured{}
	doc.SetUnstructuredContent(data)

	resourceSchema := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "nodegroups"}

	return retry.StartLoop(fmt.Sprintf("Create NodeGroup %q", nodeGroupName), 45, 15, func() error {
		res, err := client.Dynamic().Resource(resourceSchema).Create(&doc, metav1.CreateOptions{})
		if err == nil {
			logboek.LogInfoF("NodeGroup %q created\n", res.GetName())
			return nil
		}

		if errors.IsAlreadyExists(err) {
			logboek.LogInfoF("Object %v, updating...", err)
			_, err := client.Dynamic().Resource(resourceSchema).Update(&doc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			logboek.LogInfoLn("OK!")
		}
		return nil
	})
}

func WaitForKubernetesAPI(client *kube.KubernetesClient) error {
	return retry.StartLoop("Waiting for Kubernetes API to become Ready", 45, 5, func() error {
		_, err := client.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
		if err == nil {
			return nil
		}
		return fmt.Errorf("kubernetes API is not Ready: %w", err)
	})
}

func GetCloudConfig(client *kube.KubernetesClient, nodeGroupName string) (string, error) {
	var cloudData string
	err := retry.StartLoop(fmt.Sprintf("Get %q cloud config️", nodeGroupName), 45, 5, func() error {
		secret, err := client.CoreV1().Secrets("d8-cloud-instance-manager").Get("manual-bootstrap-for-"+nodeGroupName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cloudData = base64.StdEncoding.EncodeToString(secret.Data["cloud-config"])
		return nil
	})
	return cloudData, err
}

func WaitForNodesBecomeReady(client *kube.KubernetesClient, nodeGroupName string, desiredReadyNodes int) error {
	return retry.StartLoop(fmt.Sprintf("Waiting for NodeGroup %s to become Ready", nodeGroupName), 100, 20, func() error {
		nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node.deckhouse.io/group=" + nodeGroupName})
		if err != nil {
			return err
		}

		readyNodes := make(map[string]struct{})

		for _, node := range nodes.Items {
			for _, c := range node.Status.Conditions {
				if c.Type == apiv1.NodeReady {
					if c.Status == apiv1.ConditionTrue {
						readyNodes[node.Name] = struct{}{}
					}
				}
			}
		}

		if len(readyNodes) >= desiredReadyNodes {
			return nil
		}

		errorMessage := fmt.Sprintf("Nodes Ready %v of %v\n", len(readyNodes), desiredReadyNodes)
		for _, node := range nodes.Items {
			condition := "NotReady"
			if _, ok := readyNodes[node.Name]; ok {
				condition = "Ready"
			}
			errorMessage += fmt.Sprintf("* %s | %s\n", node.Name, condition)
		}

		return fmt.Errorf(errorMessage)
	})
}

func WaitForSingleNodeBecomeReady(client *kube.KubernetesClient, nodeName string) error {
	return retry.StartLoop(fmt.Sprintf("Waiting for single Node %q to become Ready", nodeName), 100, 20, func() error {
		node, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for _, c := range node.Status.Conditions {
			if c.Type == apiv1.NodeReady {
				if c.Status == apiv1.ConditionTrue {
					logboek.LogInfoLn("Success!")
					return nil
				}
			}
		}

		return fmt.Errorf("node %q is not Ready yet", nodeName)
	})
}

func IsNodeExistsInCluster(client *kube.KubernetesClient, nodeName string) (bool, error) {
	isExists := false
	err := retry.StartLoop(fmt.Sprintf("Checking that single Node %q exists", nodeName), 100, 20, func() error {
		_, err := client.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isExists = true
		return nil
	})
	return isExists, err
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
