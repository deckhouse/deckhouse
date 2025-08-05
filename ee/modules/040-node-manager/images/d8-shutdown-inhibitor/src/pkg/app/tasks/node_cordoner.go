/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

// NodeCordoner waits for shutdown signal and cordons the node.
type NodeCordoner struct {
	NodeName           string
	StartCordonCh      <-chan struct{}
	UnlockInhibitorsCh <-chan struct{}
}

func (n *NodeCordoner) Name() string {
	return "nodeCordoner"
}

func (n *NodeCordoner) Run(ctx context.Context, _ chan error) {
	// Stage 1. Wait for a signal to start.
	fmt.Printf("nodeCordoner: wait for a signal to cordon node\n")
	select {
	case <-ctx.Done():
		fmt.Printf("nodeCordoner: stop on global exit\n")
		// Return now, cordon is not needed in case of the global stop.
		return
	case <-n.StartCordonCh:
		fmt.Printf("nodeCordoner: catch a signal, cordon node\n")
	case <-n.UnlockInhibitorsCh:
		fmt.Printf("nodeCordoner: unlock signal received, cordon is not needed, exiting\n")
		return
	}

	kubectl := kubernetes.NewDefaultKubectl()
	output, err := kubectl.Cordon(n.NodeName)
	if err != nil {
		fmt.Printf("nodeCordoner: fail to cordon node: %v\n, output: %s\n", err, output)
		return
	}
	output, err = kubectl.SetCordonAnnotation(n.NodeName)
	if err != nil {
		fmt.Printf("nodeCordoner: fail set cordon annotation: %v\n, output: %s\n", err, output)
	}
}
