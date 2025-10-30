/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"log/slog"

	"d8_shutdown_inhibitor/pkg/kubernetes"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

// NodeCordoner waits for shutdown signal and cordons the node.
type NodeCordoner struct {
	NodeName           string
	StartCordonCh      <-chan struct{}
	UnlockInhibitorsCh <-chan struct{}
	CordonEnabled      bool
	Klient             *kubernetes.Klient
}

func (n *NodeCordoner) Name() string {
	return "nodeCordoner"
}

func (n *NodeCordoner) Run(ctx context.Context, _ chan error) {

    if !n.CordonEnabled {
        dlog.Info("node cordoner: cordoning disabled, skipping", slog.String("node", n.NodeName))
        return
    }

	// Stage 1. Wait for a signal to start.
	dlog.Info("node cordoner: waiting for cordon signal", slog.String("node", n.NodeName))
	select {
	case <-ctx.Done():
		dlog.Info("node cordoner: stop on context cancel", slog.String("node", n.NodeName))
		// Return now, cordon is not needed in case of the global stop.
		return
	case <-n.StartCordonCh:
		dlog.Info("node cordoner: received cordon signal", slog.String("node", n.NodeName))
	case <-n.UnlockInhibitorsCh:
		dlog.Info("node cordoner: unlock signal received, skipping cordon", slog.String("node", n.NodeName))
		return
	}

	node := n.Klient.GetNode(ctx, n.NodeName).Cordon(ctx)
	if err := node.Err(); err != nil {
		dlog.Error("node cordoner: failed to cordon node", slog.String("node", n.NodeName), dlog.Err(err))
		return
	}

	if err := n.Klient.GetNode(ctx, n.NodeName).SetCordonAnnotation(ctx).Err(); err != nil {
		dlog.Error("node cordoner: failed to set cordon annotation", slog.String("node", n.NodeName), dlog.Err(err))
	}
}
