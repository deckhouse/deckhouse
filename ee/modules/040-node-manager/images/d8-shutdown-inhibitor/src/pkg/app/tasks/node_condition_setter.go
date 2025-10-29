/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
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
	dlog.Info("node condition setter: condition updated", slog.String("node", n.NodeName))

	// Wait until inhibitors are unlocked.
	select {
	case <-ctx.Done():
		dlog.Info("node condition setter: stop on context cancel", slog.String("node", n.NodeName))
	case <-n.UnlockInhibitorsCh:
		dlog.Info("node condition setter: inhibitors unlocked, removing condition", slog.String("node", n.NodeName))
	}

	err = nc.UnsetOnUnlock(ctx, n.NodeName)
	if err != nil {
		dlog.Error("node condition setter: failed to unset condition", slog.String("node", n.NodeName), dlog.Err(err))
	}
}
