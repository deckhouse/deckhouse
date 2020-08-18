package converge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/flant/logboek"
	"github.com/hashicorp/go-multierror"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"flant/deckhouse-candi/pkg/kubernetes/actions"
	"flant/deckhouse-candi/pkg/kubernetes/actions/manifests"
	"flant/deckhouse-candi/pkg/kubernetes/client"
	"flant/deckhouse-candi/pkg/util/retry"
)

func GetNodesStateFromCluster(kubeCl *client.KubernetesClient) (map[string]map[string][]byte, error) {
	extractedState := make(map[string]map[string][]byte)
	err := retry.StartLoop("Get Nodes Terraform state from Kubernetes cluster", 45, 20, func() error {
		nodeStateSecrets, err := kubeCl.CoreV1().Secrets("d8-system").List(metav1.ListOptions{LabelSelector: "node.deckhouse.io/terraform-state"})
		if err != nil {
			return err
		}

		for _, nodeState := range nodeStateSecrets.Items {
			name := nodeState.Labels["node.deckhouse.io/node-name"]
			if name == "" {
				return fmt.Errorf("can't determine Node name for %q secret", nodeState.Name)
			}

			nodeGroup := nodeState.Labels["node.deckhouse.io/node-group"]
			if nodeGroup == "" {
				return fmt.Errorf("can't determine NodeGroup for %q secret", nodeState.Name)
			}
			if extractedState[nodeGroup] == nil {
				extractedState[nodeGroup] = make(map[string][]byte)
			}

			state := nodeState.Data["node-tf-state.json"]
			extractedState[nodeGroup][name] = state
			logboek.LogInfoF("nodeGroup=%s nodeName=%s symbols=%v\n", nodeGroup, name, len(state))
		}
		return nil
	})
	return extractedState, err
}

func GetClusterStateFromCluster(kubeCl *client.KubernetesClient) ([]byte, error) {
	var state []byte
	err := retry.StartLoop("Get Cluster Terraform state from Kubernetes cluster", 45, 20, func() error {
		clusterStateSecret, err := kubeCl.CoreV1().Secrets("d8-system").Get("d8-cluster-terraform-state", metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				// Return empty state, if there is no state in cluster. Need to skip cluster state apply in converge.
				return nil
			}
			return err
		}

		state = clusterStateSecret.Data["cluster-tf-state.json"]
		return nil
	})
	return state, err
}

func IsNodeExistsInCluster(kubeCl *client.KubernetesClient, nodeName string) (bool, error) {
	isExists := false
	err := retry.StartLoop(fmt.Sprintf("Checking that single Node %q exists", nodeName), 100, 20, func() error {
		_, err := kubeCl.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isExists = true
		return nil
	})
	return isExists, err
}

func WaitForSingleNodeBecomeReady(kubeCl *client.KubernetesClient, nodeName string) error {
	return retry.StartLoop(fmt.Sprintf("Waiting for single Node %q to become Ready", nodeName), 100, 20, func() error {
		node, err := kubeCl.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		for _, c := range node.Status.Conditions {
			if c.Type == apiv1.NodeReady {
				if c.Status == apiv1.ConditionTrue {
					return nil
				}
			}
		}

		return fmt.Errorf("node %q is not Ready yet", nodeName)
	})
}

func WaitForNodesBecomeReady(kubeCl *client.KubernetesClient, nodeGroupName string, desiredReadyNodes int) error {
	return retry.StartLoop(fmt.Sprintf("Waiting for NodeGroup %s to become Ready", nodeGroupName), 100, 20, func() error {
		nodes, err := kubeCl.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: "node.deckhouse.io/group=" + nodeGroupName})
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

func SaveNodeTerraformState(kubeCl *client.KubernetesClient, nodeName, nodeGroup string, tfState []byte) error {
	getManifest := func() interface{} { return manifests.SecretWithNodeTerraformState(nodeName, nodeGroup, tfState) }
	return retry.StartLoop(fmt.Sprintf("Save Terraform state for Node %q", nodeName), 45, 10, func() error {
		task := actions.ManifestTask{
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
		}
		return task.Create()
	})
}

func SaveMasterNodeTerraformState(kubeCl *client.KubernetesClient, nodeName string, tfState, devicePath []byte) error {
	getTerraformStateManifest := func() interface{} {
		return manifests.SecretWithNodeTerraformState(nodeName, masterNodeGroupName, tfState)
	}
	getDevicePathManifest := func() interface{} {
		return manifests.SecretMasterDevicePath(nodeName, devicePath)
	}
	return retry.StartLoop(fmt.Sprintf("Save Terraform state for master Node %q", nodeName), 45, 10, func() error {
		tasks := []actions.ManifestTask{
			{
				Name:     fmt.Sprintf(`Secret "d8-node-terraform-state-%s"`, nodeName),
				Manifest: getTerraformStateManifest,
				CreateFunc: func(manifest interface{}) error {
					_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
					return err
				},
				UpdateFunc: func(manifest interface{}) error {
					_, err := kubeCl.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
					return err
				},
			},
			{
				Name:     "d8-masters-kubernetes-data-device-path",
				Manifest: getDevicePathManifest,
				CreateFunc: func(manifest interface{}) error {
					_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
					return err
				},
				UpdateFunc: func(manifest interface{}) error {
					data, err := json.Marshal(manifest.(*apiv1.Secret))
					if err != nil {
						return err
					}
					_, err = kubeCl.CoreV1().Secrets("d8-system").Patch(
						"d8-masters-kubernetes-data-device-path",
						types.MergePatchType,
						data,
					)
					return err
				},
			},
		}

		var allErrs *multierror.Error
		for _, task := range tasks {
			if err := task.Create(); err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		}
		return allErrs.ErrorOrNil()
	})
}

func DeleteTerraformState(kubeCl *client.KubernetesClient, secretName string) error {
	return retry.StartLoop(fmt.Sprintf("Save Terraform %q", secretName), 45, 10, func() error {
		return kubeCl.CoreV1().Secrets("d8-system").Delete(secretName, &metav1.DeleteOptions{})
	})
}

func SaveClusterTerraformState(kubeCl *client.KubernetesClient, tfState []byte) error {
	return retry.StartLoop("Save Cluster Terraform state", 45, 10, func() error {
		task := actions.ManifestTask{
			Name:     `Secret "d8-cluster-terraform-state"`,
			Manifest: func() interface{} { return manifests.SecretWithTerraformState(tfState) },
			CreateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Create(manifest.(*apiv1.Secret))
				return err
			},
			UpdateFunc: func(manifest interface{}) error {
				_, err := kubeCl.CoreV1().Secrets("d8-system").Update(manifest.(*apiv1.Secret))
				return err
			},
		}
		return task.Create()
	})
}

func GetCloudConfig(kubeCl *client.KubernetesClient, nodeGroupName string) (string, error) {
	var cloudData string
	err := retry.StartLoop(fmt.Sprintf("Get %q cloud config️", nodeGroupName), 45, 5, func() error {
		secret, err := kubeCl.CoreV1().Secrets("d8-cloud-instance-manager").Get("manual-bootstrap-for-"+nodeGroupName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		cloudData = base64.StdEncoding.EncodeToString(secret.Data["cloud-config"])
		return nil
	})
	return cloudData, err
}

func CreateNodeGroup(kubeCl *client.KubernetesClient, nodeGroupName string, data map[string]interface{}) error {
	doc := unstructured.Unstructured{}
	doc.SetUnstructuredContent(data)

	resourceSchema := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "nodegroups"}

	return retry.StartLoop(fmt.Sprintf("Create NodeGroup %q", nodeGroupName), 45, 15, func() error {
		res, err := kubeCl.Dynamic().Resource(resourceSchema).Create(&doc, metav1.CreateOptions{})
		if err == nil {
			logboek.LogInfoF("NodeGroup %q created\n", res.GetName())
			return nil
		}

		if errors.IsAlreadyExists(err) {
			logboek.LogInfoF("Object %v, updating...", err)
			_, err := kubeCl.Dynamic().Resource(resourceSchema).Update(&doc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			logboek.LogInfoLn("OK!")
		}
		return nil
	})
}
