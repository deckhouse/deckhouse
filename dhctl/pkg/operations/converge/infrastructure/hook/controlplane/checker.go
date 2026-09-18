// Copyright 2026 Flant JSC
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

	libcon "github.com/deckhouse/lib-connection/pkg"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
)

type Checker struct {
	nodeToHostForChecks map[string]string
	checkers            []hook.NodeChecker
	confirm             ConfirmFunc
}

type ConfirmFunc func(msg string) bool

var DefaultConfirm = ConfirmFunc(func(msg string) bool {
	return true
})

func NewChecker(nodeToHostForChecks map[string]string, checkers []hook.NodeChecker, confirm ConfirmFunc) *Checker {
	return &Checker{
		nodeToHostForChecks: nodeToHostForChecks,
		checkers:            checkers,
		confirm:             confirm,
	}
}

func (c *Checker) IsAllNodesReady(ctx context.Context) error {
	if c.checkers == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("No checkers passed. Skipping. Nodes to check: %v", c.nodeToHostForChecks))

		return nil
	}

	if len(c.nodeToHostForChecks) == 0 {
		return fmt.Errorf("no nodes provided for the control-plane nodes readiness check")
	}

	for nodeName := range c.nodeToHostForChecks {
		if !c.confirm(fmt.Sprintf("Do you want to wait for node %s to become ready?", nodeName)) {
			continue
		}

		ready, err := hook.IsNodeReady(ctx, c.checkers, nodeName)
		if err != nil {
			return err
		}

		if !ready {
			return hook.ErrNotReady
		}
	}

	return nil
}

// NewControlPlaneChecker builds the gate that stands in front of every destructive action
// on a master: the nodes named here are the ones that have to survive it.
func NewControlPlaneChecker(
	kubeClientProvider kubernetes.KubeClientProviderWithCtx,
	sshProvider libcon.SSHProvider,
	nodeToHostForChecks map[string]string,
	commanderMode bool,
	skipChecks bool,
	immutableNode bool,
) *Checker {
	checkers := []hook.NodeChecker{
		hook.NewKubeNodeReadinessChecker(kubeClientProvider),
	}

	// An immutable node answers no sshd: the check would fail on every master, and
	// what it proves — that the machine is alive and serving — the control plane
	// checker below proves through the cluster.
	if !commanderMode && !skipChecks && !immutableNode {
		checkers = append(checkers, NewSSHChecker(sshProvider, nodeToHostForChecks))
	}
	checkers = append(checkers, NewManagerReadinessChecker(kubeClientProvider))
	checkers = append(checkers, NewStrongholdReadinessChecker(kubeClientProvider))

	return NewChecker(nodeToHostForChecks, checkers, DefaultConfirm)
}
