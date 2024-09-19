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

package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	flantkubeclient "github.com/flant/kube-client/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type HookForDestroyPipeline struct {
	kubeCl            *client.KubernetesClient
	nodeToDestroy     string
	oldMasterIPForSSH string
}

func NewHookForDestroyPipeline(kubeCl *client.KubernetesClient, nodeToDestroy string) *HookForDestroyPipeline {
	return &HookForDestroyPipeline{
		kubeCl:        kubeCl,
		nodeToDestroy: nodeToDestroy,
	}
}

func (h *HookForDestroyPipeline) BeforeAction(runner terraform.RunnerInterface) (bool, error) {
	outputs, err := terraform.GetMasterNodeResult(runner)
	if err != nil {
		log.ErrorF("Get master node pipeline outputs: %v", err)
	}

	h.oldMasterIPForSSH = outputs.MasterIPForSSH

	err = removeControlPlaneRoleFromNode(h.kubeCl, h.nodeToDestroy)
	if err != nil {
		return false, fmt.Errorf("failed to remove control plane role from node '%s': %v", h.nodeToDestroy, err)
	}

	return false, nil
}

func (h *HookForDestroyPipeline) AfterAction(runner terraform.RunnerInterface) error {
	cl := h.kubeCl.NodeInterfaceAsSSHClient()
	if cl == nil {
		log.DebugLn("Node interface is not ssh")
		return nil
	}

	cl.Settings.RemoveAvailableHosts(h.oldMasterIPForSSH)
	return nil
}

func (h *HookForDestroyPipeline) IsReady() error {
	return nil
}

func removeControlPlaneRoleFromNode(kubeCl *client.KubernetesClient, nodeName string) error {
	err := removeLabelsFromNode(kubeCl, nodeName, []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		"node.deckhouse.io/group",
	})
	if err != nil {
		return fmt.Errorf("failed to remove labels from node '%s': %v", nodeName, err)
	}

	err = waitEtcdHasNoMember(kubeCl.KubeClient.(*flantkubeclient.Client), nodeName)
	if err != nil {
		return fmt.Errorf("failed to check etcd has no member '%s': %v", nodeName, err)
	}

	err = tryToDrainNode(kubeCl, nodeName)
	if err != nil {
		return fmt.Errorf("failed to drain node '%s': %v", nodeName, err)
	}

	return nil
}

func removeLabelsFromNode(kubeCl *client.KubernetesClient, nodeName string, labels []string) error {
	return retry.NewLoop(fmt.Sprintf("Remove labels from node %s", nodeName), 45, 5*time.Second).Run(func() error {
		node, err := kubeCl.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		nodeLabels := node.GetLabels()

		patchOperations := make([]map[string]interface{}, 0, len(labels))

		for _, label := range labels {
			// Check if the label exists on the node before trying to remove it
			_, ok := nodeLabels[label]
			if !ok {
				continue
			}

			patchOperations = append(patchOperations, map[string]interface{}{
				"op": "remove",
				// JSON patch requires slashes to be escaped with ~1
				"path": fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(label, "/", "~1")),
			})
		}

		if len(patchOperations) == 0 {
			return nil
		}

		patch, err := json.Marshal(patchOperations)
		if err != nil {
			return err
		}

		_, err = kubeCl.CoreV1().Nodes().Patch(context.TODO(), nodeName, types.JSONPatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}

		return nil
	})
}
