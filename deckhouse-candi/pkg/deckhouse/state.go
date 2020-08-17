package deckhouse

import (
	"strings"

	"github.com/flant/logboek"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/util/retry"
)

func GetNodesStateFromCluster(client *kube.KubernetesClient) (map[string]map[string][]byte, error) {
	extractedState := make(map[string]map[string][]byte)
	err := retry.StartLoop("Get Nodes Terraform state from Kubernetes cluster", 45, 20, func() error {
		nodeStateSecrets, err := client.CoreV1().Secrets("d8-system").List(metav1.ListOptions{LabelSelector: "node.deckhouse.io/terraform-state"})
		if err != nil {
			return err
		}

		for _, nodeState := range nodeStateSecrets.Items {
			name := strings.TrimPrefix(nodeState.Name, "d8-node-terraform-state-")
			nodeGroup := nodeState.Labels["node.deckhouse.io/terraform-state"]
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

func GetClusterStateFromCluster(client *kube.KubernetesClient) ([]byte, error) {
	var state []byte
	err := retry.StartLoop("Get Cluster Terraform state from Kubernetes cluster", 45, 20, func() error {
		clusterStateSecret, err := client.CoreV1().Secrets("d8-system").Get("d8-cluster-terraform-state", metav1.GetOptions{})
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
