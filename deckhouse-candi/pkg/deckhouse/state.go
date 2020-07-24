package deckhouse

import (
	"fmt"
	"strings"
	"time"

	"github.com/flant/logboek"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"flant/deckhouse-candi/pkg/kube"
	"flant/deckhouse-candi/pkg/log"
)

func GetNodesStateFromCluster(client *kube.KubernetesClient) (map[string]map[string][]byte, error) {
	extractedState := make(map[string]map[string][]byte)
	err := logboek.LogProcess("Get cluster nodes terraform state from cluster", log.BoldOptions(), func() error {
		for i := 1; i <= 45; i++ {
			nodeStateSecrets, err := client.CoreV1().Secrets("d8-system").List(metav1.ListOptions{LabelSelector: "node.deckhouse.io/terraform-state"})
			if err != nil {
				logboek.LogInfoF("[Attempt #%v of 100] Error while getting nodes state, next attempt will be in 10s\n", i)
				logboek.LogInfoF("%v\n\n", err)
				time.Sleep(10 * time.Second)
				continue
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
		}
		return fmt.Errorf("failed getting nodes terraform state")
	})
	return extractedState, err
}

func GetClusterStateFromCluster(client *kube.KubernetesClient) ([]byte, error) {
	var state []byte
	err := logboek.LogProcess("Get cluster terraform state from cluster", log.BoldOptions(), func() error {
		for i := 1; i <= 45; i++ {
			clusterStateSecret, err := client.CoreV1().Secrets("d8-system").Get("d8-cluster-terraform-state", metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Return empty state, if there is no state in cluster. Need to skip cluster state apply in converge.
					return nil
				}
				logboek.LogInfoF("[Attempt #%v of 100] Error while getting cluster state, next attempt will be in 10s\n", i)
				logboek.LogInfoF("%v\n\n", err)
				time.Sleep(10 * time.Second)
				continue
			}

			state = clusterStateSecret.Data["cluster-tf-state.json"]
			logboek.LogInfoF("Received %v symbols\n", len(state))
			return nil
		}
		return fmt.Errorf("failed getting cluster terraform state")
	})
	return state, err
}
