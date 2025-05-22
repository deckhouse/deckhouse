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

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
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

func (n *NodeConditionSetter) Run(ctx context.Context, errCh chan error) {
	err := nodecondition.GracefulShutdownPostpone().SetOnStart(n.NodeName)
	if err != nil {
		errCh <- fmt.Errorf("nodeConditionSetter patch Node to set condition: %w", err)
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

	err = nodecondition.GracefulShutdownPostpone().UnsetOnUnlock(n.NodeName)
	fmt.Printf("nodeConditionSetter(s2): failed to unset condition on Node: %v\n", err)
}
