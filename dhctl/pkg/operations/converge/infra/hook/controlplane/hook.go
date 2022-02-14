// Copyright 2021 Flant JSC
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
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/manifests"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

type Hook struct {
	nodesNamesToCheck []string
	checkers          []hook.NodeChecker
	sourceCommandName string
	kubeCl            *client.KubernetesClient
	nodeToConverge    string
	runAfterAction    bool
}

func NewHook(kubeCl *client.KubernetesClient, nodesToCheckWithIPs map[string]string, clusterUUID string) *Hook {
	proxyChecker := NewKubeProxyChecker().
		WithExternalIPs(nodesToCheckWithIPs).
		WithClusterUUID(clusterUUID)

	checkers := []hook.NodeChecker{
		hook.NewKubeNodeReadinessChecker(kubeCl),
		proxyChecker,
		NewManagerReadinessChecker(kubeCl),
	}

	nodes := make([]string, 0)
	for nodeName := range nodesToCheckWithIPs {
		nodes = append(nodes, nodeName)
	}

	return &Hook{
		nodesNamesToCheck: nodes,
		checkers:          checkers,
		kubeCl:            kubeCl,
	}
}

func (h *Hook) WithSourceCommandName(name string) *Hook {
	h.sourceCommandName = name
	return h
}

func (h *Hook) WithNodeToConverge(nodeToConverge string) *Hook {
	h.nodeToConverge = nodeToConverge
	return h
}

func (h *Hook) convergeLabelToNode(shouldExist bool) error {
	node, err := h.kubeCl.CoreV1().Nodes().Get(context.TODO(), h.nodeToConverge, metav1.GetOptions{})
	if err != nil {
		return err
	}

	labels := node.GetLabels()

	if shouldExist {
		if _, ok := labels[manifests.ConvergeLabel]; ok {
			return nil
		}

		labels[manifests.ConvergeLabel] = ""
	} else {
		if _, ok := labels[manifests.ConvergeLabel]; !ok {
			return nil
		}

		delete(labels, manifests.ConvergeLabel)
	}

	node.SetLabels(labels)

	_, err = h.kubeCl.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})

	return err
}

func (h *Hook) BeforeAction() (bool, error) {
	err := log.Process(h.sourceCommandName, "Check deckhouse pod is not on converged node", func() error {
		var pod *v1.Pod
		err := retry.NewSilentLoop("Get deckhouse pod", 10, 3*time.Second).Run(func() error {
			var err error
			pod, err = deckhouse.GetRunningPod(h.kubeCl)

			return err
		})

		if err != nil {
			return fmt.Errorf("Deckhouse pod did not get: %s", err)
		}

		if pod.Spec.NodeName != h.nodeToConverge {
			h.runAfterAction = false
			return nil
		}

		confirm := input.NewConfirmation().
			WithMessage("Deckhouse pod is located on node to converge. Do you want to move pod in another node?")

		if !confirm.Ask() {
			log.WarnLn("Skip moving deckhouse pod")
			h.runAfterAction = false
			return nil
		}

		title := fmt.Sprintf("Set label '%s' on converged node", manifests.ConvergeLabel)
		err = retry.NewLoop(title, 10, 3*time.Second).Run(func() error {
			return h.convergeLabelToNode(true)
		})

		if err != nil {
			return fmt.Errorf("Cannot set label '%s' to node: %v", manifests.ConvergeLabel, err)
		}

		err = retry.NewLoop("Evict deckhouse pod from node", 10, 3*time.Second).Run(func() error {
			return deckhouse.DeletePod(h.kubeCl)
		})

		if err != nil {
			return err
		}

		h.runAfterAction = true

		return nil
	})

	if err != nil {
		return false, err
	}

	return h.runAfterAction, err
}

func (h *Hook) AfterAction() error {
	if !h.runAfterAction {
		return nil
	}

	title := fmt.Sprintf("Delete label '%s' from converged node", manifests.ConvergeLabel)
	return retry.NewLoop(title, 10, 3*time.Second).Run(func() error {
		return h.convergeLabelToNode(false)
	})
}

func (h *Hook) IsReady() error {
	excludeNode := h.nodeToConverge
	if !h.runAfterAction {
		excludeNode = ""
	}

	err := deckhouse.WaitForReadinessNotOnNode(h.kubeCl, excludeNode)
	if err != nil {
		return err
	}

	return hook.IsAllNodesReady(h.checkers, h.nodesNamesToCheck, h.sourceCommandName, "Control plane nodes are ready")
}
