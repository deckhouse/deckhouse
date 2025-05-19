/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tasks

import (
	"context"
	"fmt"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

// NodeConditionSetter set condition on start to prevent shutdown sequence in the kubelet
// and remove it when inhibitors are unlocked so kubelet continue its shutdown sequence.
type NodeConditionSetter struct {
	NodeName string

	// UnlockInhibitorsCh is a channel to get event about unlocking inhibitors.
	UnlockInhibitorsCh <-chan struct{}
}

func (n *NodeConditionSetter) Name() string {
	return "nodeConditionSetter"
}

const (
	ConditionType        = "GracefulShutdownPostpone"
	ConditionStatusTrue  = "True"
	ConditionStatusFalse = "False"
)

func (n *NodeConditionSetter) Run(ctx context.Context, errCh chan error) {
	err := n.patchCondition(ConditionStatusTrue)
	if err != nil {
		errCh <- fmt.Errorf("nodeConditionSetter set Node condition: %w", err)
		return
	}
	fmt.Printf("nodeConditionSetter(s1): Node condition updated\n")

	// Wait until inhibitors are unlocked.
	select {
	case <-ctx.Done():
		fmt.Printf("nodeConditionSetter(s2): stop on global exit\n")
	case <-n.UnlockInhibitorsCh:
		fmt.Printf("nodeConditionSetter(s2): inhibitors unlocked, unset Node condition\n")
	}

	err = n.patchCondition(ConditionStatusFalse)
	fmt.Printf("nodeConditionSetter(s2): failed to unset condition on Node: %v\n", err)
}

/**

kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"True", "reason":"PodsWithLabelAreRunningOnNode"}]}}'
--subresource=status

kubectl patch node/static-vm-node-00 --type strategic
-p '{"status":{"conditions":[{"type":"GracefulShutdownPostpone", "status":"False", "reason":"NoRunningPodsWithLabel"}]}}'
--subresource=status
*/
func (n *NodeConditionSetter) patchCondition(status string) error {
	reason := ""
	switch status {
	case ConditionStatusTrue:
		reason = "PodsWithLabelAreRunningOnNode"
	case ConditionStatusFalse:
		reason = "NoRunningPodsWithLabel"
	}
	k := kubernetes.NewDefaultKubectl()
	return k.PatchCondition("Node", n.NodeName, ConditionType, status, reason, "")
}
