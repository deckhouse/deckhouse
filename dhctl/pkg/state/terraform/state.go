// Copyright 2024 Flant JSC
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

package terraform

import (
	"context"
	"fmt"
	"time"

	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

func GetClusterStateFromCluster(kubeCl *client.KubernetesClient) ([]byte, error) {
	var state []byte
	err := retry.NewLoop("Get Cluster Terraform state from Kubernetes cluster", 5, 5*time.Second).Run(func() error {
		clusterStateSecret, err := kubeCl.CoreV1().Secrets("d8-system").Get(context.TODO(), "d8-cluster-terraform-state", metav1.GetOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
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

func GetClusterUUID(kubeCl *client.KubernetesClient) (string, error) {
	var clusterUUID string
	err := retry.NewLoop("Get Cluster UUID from the Kubernetes cluster", 5, 5*time.Second).Run(func() error {
		uuidConfigMap, err := kubeCl.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "d8-cluster-uuid", metav1.GetOptions{})
		if err != nil {
			return err
		}

		clusterUUID = uuidConfigMap.Data["cluster-uuid"]
		return nil
	})
	return clusterUUID, err
}

func GetNodesStateFromCluster(kubeCl *client.KubernetesClient) (map[string]state.NodeGroupTerraformState, error) {
	extractedState := make(map[string]state.NodeGroupTerraformState)

	err := retry.NewLoop("Get Nodes Terraform state from Kubernetes cluster", 5, 5*time.Second).Run(func() error {
		nodeStateSecrets, err := kubeCl.CoreV1().Secrets("d8-system").List(context.TODO(), metav1.ListOptions{LabelSelector: "node.deckhouse.io/terraform-state"})
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

			if _, ok := extractedState[nodeGroup]; !ok {
				extractedState[nodeGroup] = state.NodeGroupTerraformState{State: make(map[string][]byte)}
			}

			// TODO: validate, that all secrets from node group have same node-group-settings.json
			nodeGroupTerraformState := extractedState[nodeGroup]
			nodeGroupTerraformState.Settings = nodeState.Data["node-group-settings.json"]

			state := nodeState.Data["node-tf-state.json"]
			nodeGroupTerraformState.State[name] = state

			extractedState[nodeGroup] = nodeGroupTerraformState
		}
		return nil
	})
	return extractedState, err
}
