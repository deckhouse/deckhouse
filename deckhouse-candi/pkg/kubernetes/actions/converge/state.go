package converge

import (
	"encoding/json"
	"fmt"

	"github.com/flant/logboek"
	"github.com/hashicorp/go-multierror"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return retry.StartLoop(fmt.Sprintf("Save Terraform state for master Node %s", nodeName), 45, 10, func() error {
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
				Name:     `Secret "d8-masters-kubernetes-data-device-path"`,
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

func DeleteTerraformState(kubeCl *client.KubernetesClient, secretName string) error {
	return retry.StartLoop(fmt.Sprintf("Save Terraform %q", secretName), 45, 10, func() error {
		return kubeCl.CoreV1().Secrets("d8-system").Delete(secretName, &metav1.DeleteOptions{})
	})
}
