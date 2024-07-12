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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infra/hook"
)

type Hook struct {
	nodesNamesToCheck []string
	checkers          []hook.NodeChecker
	sourceCommandName string
	kubeCl            *client.KubernetesClient
	nodeToConverge    string
	runAfterAction    bool
	confirm           func(msg string) bool
}

func NewHook(kubeCl *client.KubernetesClient, nodeToHostForChecks map[string]string, clusterUUID string) *Hook {
	addProxyChecker := true
	nodes := make([]string, 0)

	for nodeName, host := range nodeToHostForChecks {
		nodes = append(nodes, nodeName)
		if host == "" {
			addProxyChecker = false
		}
	}

	checkers := []hook.NodeChecker{
		hook.NewKubeNodeReadinessChecker(kubeCl),
	}

	if addProxyChecker {
		proxyChecker := NewKubeProxyChecker().
			WithExternalIPs(nodeToHostForChecks).
			WithClusterUUID(clusterUUID)
		checkers = append(checkers, proxyChecker)
	}

	checkers = append(checkers, NewManagerReadinessChecker(kubeCl))

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

func (h *Hook) WithConfirm(confirm func(msg string) bool) *Hook {
	h.confirm = confirm
	return h
}

func (h *Hook) BeforeAction() (bool, error) {
	return false, nil
}

func (h *Hook) AfterAction() error {
	return nil
}

func (h *Hook) runConfirm(msg string) bool {
	confirm := true

	if h.confirm != nil {
		confirm = h.confirm(msg)
	}

	return confirm
}

func (h *Hook) isAllNodesReady() error {
	if h.checkers == nil {
		log.DebugF("Not passed checkers. Skip. Nodes for check: %v", h.nodesNamesToCheck)
		return nil
	}

	if len(h.nodesNamesToCheck) == 0 {
		return fmt.Errorf("Do not have nodes for control plane nodes are readinss check.")
	}

	for _, nodeName := range h.nodesNamesToCheck {
		if !h.runConfirm(fmt.Sprintf("Do you want to wait node %s will be ready?", nodeName)) {
			continue
		}

		ready, err := hook.IsNodeReady(h.checkers, nodeName, h.sourceCommandName)
		if err != nil {
			return err
		}

		if !ready {
			return hook.ErrNotReady
		}
	}

	return nil
}

func (h *Hook) IsReady() error {
	return h.isAllNodesReady()
}
