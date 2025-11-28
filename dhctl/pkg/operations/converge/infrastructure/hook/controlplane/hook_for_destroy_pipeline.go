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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	infra_utils "github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/utils"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type HookForDestroyPipeline struct {
	getter            kubernetes.KubeClientProvider
	nodeToDestroy     string
	oldMasterIPForSSH string
	commanderMode     bool
}

func NewHookForDestroyPipeline(getter kubernetes.KubeClientProvider, nodeToDestroy string, commanderMode bool) *HookForDestroyPipeline {
	return &HookForDestroyPipeline{
		getter:        getter,
		nodeToDestroy: nodeToDestroy,
		commanderMode: commanderMode,
	}
}

func (h *HookForDestroyPipeline) BeforeAction(ctx context.Context, runner infrastructure.RunnerInterface) (bool, error) {
	outputs, err := infrastructure.GetMasterNodeResult(ctx, runner)
	if err != nil {
		return false, fmt.Errorf("Get master node pipeline outputs got error: %w", err)
	}

	masterIP := outputs.MasterIPForSSH
	if masterIP == "" {
		log.InfoF("Got empty master IP for ssh for node %s. Skip removing control-plane from node.\n", h.nodeToDestroy)
		return false, nil
	}

	h.oldMasterIPForSSH = masterIP

	err = removeControlPlaneRoleFromNode(ctx, h.getter.KubeClient(), h.nodeToDestroy, h.commanderMode)
	if err != nil {
		return false, fmt.Errorf("failed to remove control plane role from node '%s': %v", h.nodeToDestroy, err)
	}

	return false, nil
}

func (h *HookForDestroyPipeline) AfterAction(_ context.Context, runner infrastructure.RunnerInterface) error {
	if h.commanderMode {
		return nil
	}

	cl := h.getter.KubeClient().NodeInterfaceAsSSHClient()
	if cl == nil {
		log.DebugLn("Node interface is not ssh")
		return nil
	}

	if h.oldMasterIPForSSH != "" {
		cl.Session().RemoveAvailableHosts(session.Host{Host: h.oldMasterIPForSSH, Name: h.nodeToDestroy})
	}

	return nil
}

func (h *HookForDestroyPipeline) IsReady() error {
	return nil
}

func removeControlPlaneRoleFromNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, commanderMode bool) error {
	err := removeLabelsFromNode(ctx, kubeCl, nodeName, []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		"node.deckhouse.io/group",
	})
	if err != nil {
		return fmt.Errorf("failed to remove labels from node '%s': %v", nodeName, err)
	}

	err = waitEtcdHasNoMember(ctx, kubeCl.KubeClient.(*flantkubeclient.Client), nodeName)
	if err != nil {
		return fmt.Errorf("failed to check etcd has no member '%s': %v", nodeName, err)
	}

	err = infra_utils.TryToDrainNode(ctx, kubeCl, nodeName, infra_utils.GetDrainConfirmation(commanderMode), infra_utils.DrainOptions{Force: true})
	if err != nil {
		return fmt.Errorf("failed to drain node '%s': %v", nodeName, err)
	}

	return nil
}

func removeLabelsFromNode(ctx context.Context, kubeCl *client.KubernetesClient, nodeName string, labels []string) error {
	return retry.NewLoop(fmt.Sprintf("Remove labels from node %s", nodeName), 45, 5*time.Second).RunContext(ctx, func() error {
		node, err := kubeCl.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				log.InfoF("Node '%s' has been deleted. Skip\n", nodeName)
				return nil
			}
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

		_, err = kubeCl.CoreV1().Nodes().Patch(ctx, nodeName, types.JSONPatchType, patch, metav1.PatchOptions{})
		if err != nil {
			return err
		}

		return nil
	})
}
