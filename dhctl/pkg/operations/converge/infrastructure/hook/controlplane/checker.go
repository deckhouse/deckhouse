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
	"fmt"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook"
)

type Checker struct {
	nodeToHostForChecks map[string]string
	checkers            []hook.NodeChecker
	sourceCommandName   string
	confirm             ConfirmFunc
}

type ConfirmFunc func(msg string) bool

var DefaultConfirm = ConfirmFunc(func(msg string) bool {
	return true
})

func NewChecker(nodeToHostForChecks map[string]string, checkers []hook.NodeChecker, sourceCommandName string, confirm ConfirmFunc) *Checker {
	return &Checker{
		nodeToHostForChecks: nodeToHostForChecks,
		checkers:            checkers,
		sourceCommandName:   sourceCommandName,
		confirm:             confirm,
	}
}

func (c *Checker) IsAllNodesReady(ctx context.Context) error {
	if c.checkers == nil {
		log.DebugF("Not passed checkers. Skip. Nodes for check: %v", c.nodeToHostForChecks)

		return nil
	}

	if len(c.nodeToHostForChecks) == 0 {
		return fmt.Errorf("do not have nodes for control plane nodes are readinss check")
	}

	for nodeName := range c.nodeToHostForChecks {
		if !c.confirm(fmt.Sprintf("Do you want to wait node %s will be ready?", nodeName)) {
			continue
		}

		ready, err := hook.IsNodeReady(ctx, c.checkers, nodeName, c.sourceCommandName)
		if err != nil {
			return err
		}

		if !ready {
			return hook.ErrNotReady
		}
	}

	return nil
}
