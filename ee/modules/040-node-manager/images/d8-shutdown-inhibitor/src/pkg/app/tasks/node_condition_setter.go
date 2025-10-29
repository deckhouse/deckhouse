/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"
)

// NodeConditionSetter set condition on start to prevent shutdown sequence in the kubelet
// and remove it when inhibitors are unlocked so kubelet continue its shutdown sequence.
type NodeConditionSetter struct {
	NodeName string
	Klient   *kubernetes.Klient
	// UnlockInhibitorsCh is a channel to get event about unlocking inhibitors.
	UnlockInhibitorsCh <-chan struct{}
}

func (n *NodeConditionSetter) Name() string {
	return "nodeConditionSetter"
}

func (n *NodeConditionSetter) Run(ctx context.Context, errCh chan error) {
	nc := nodecondition.GracefulShutdownPostpone(n.Klient)

	err := nc.SetOnStart(ctx, n.NodeName)
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

	err = nc.UnsetOnUnlock(ctx, n.NodeName)
	if err != nil {
		fmt.Printf("nodeConditionSetter(s2): failed to unset condition on Node: %v\n", err)
	}
}
