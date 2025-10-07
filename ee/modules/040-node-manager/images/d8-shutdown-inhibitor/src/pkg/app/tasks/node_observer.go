/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"d8_shutdown_inhibitor/pkg/kubernetes"
)

const (
	MasterNgName = "master"
)

type NodeInhibitorDecision struct {
	Enable bool
}

// NodeObserver waits for the shutdown signal, checks whether the node
// should keep the shutdown inhibitor active, and shares the decision with
// other tasks.
type NodeObserver struct {
	NodeName                string
	ShutdownSignalCh        <-chan struct{}
	InhibitorDecisionCh     chan<- NodeInhibitorDecision
	StopInhibitorsCh        chan<- struct{}
	NodeGroup               string
	NodeCheckingInterval    time.Duration
	MaxNodeObserverAttempts int
}

func (p *NodeObserver) Name() string {
	return "nodeObserver"
}

func (p *NodeObserver) Run(ctx context.Context, errCh chan error) {
	if p.InhibitorDecisionCh == nil {
		fmt.Printf("nodeObserver: decision channel is nil, nothing to notify\n")
		return
	}
	defer close(p.InhibitorDecisionCh)

	fmt.Printf("nodeObserver: wait for shutdown signal\n")
	select {
	case <-ctx.Done():
		fmt.Printf("nodeObserver: stop on context cancel\n")
		return
	case <-p.ShutdownSignalCh:
		fmt.Printf("nodeObserver: catch prepare shutdown signal, start node checker\n")
	}

	ticker := time.NewTicker(p.NodeCheckingInterval)
	defer ticker.Stop()

	attempt := 0

	for {
		attempt++
		enable, err := p.ShouldEnableInhibitor(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			fmt.Printf("nodeObserver: inhibitor check failed (attempt %d/%d): %v\n", attempt, p.MaxNodeObserverAttempts, err)
			if attempt >= p.MaxNodeObserverAttempts {
				// main priority is podObserver
				fmt.Printf("nodeObserver: reached max attempts, keeping inhibitors enabled and exiting\n")
				p.InhibitorDecisionCh <- NodeInhibitorDecision{Enable: true}
				return
			}
			select {
			case <-ctx.Done():
				fmt.Printf("nodeObserver: stop on context cancel\n")
				return
			case <-ticker.C:
			}
			continue
		}

		p.InhibitorDecisionCh <- NodeInhibitorDecision{Enable: enable}
		fmt.Printf("nodeObserver: inhibitor decision for node %q: enable=%t\n", p.NodeName, enable)
		return
	}
}

func (p *NodeObserver) ShouldEnableInhibitor(ctx context.Context) (bool, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false, ctxErr
	}

	defaultKubectl := kubernetes.NewDefaultKubectl()
	ngName, err := defaultKubectl.GetNodeGroup(p.NodeName)
	switch {
	case errors.Is(err, kubernetes.ErrNodeIsNotNgManaged):
		fmt.Printf("nodeObserver: node %q is not managed by Deckhouse node group label\n", p.NodeName)
		return false, nil
	case err != nil:
		return false, err
	}

	p.NodeGroup = ngName
	fmt.Printf("nodeObserver: node %q detected in node group %q\n", p.NodeName, ngName)

	if !p.isMasterGroup() {
		fmt.Printf("nodeObserver: node group %q is not master, inhibitors remain enabled\n", ngName)
		return true, nil
	}

	adminKubectl := kubernetes.NewAdmintKubectl()
	nodesCount, err := adminKubectl.CountNodes()
	if err != nil {
		return false, err
	}
	fmt.Printf("nodeObserver: cluster total nodes count = %d\n", nodesCount)
	return nodesCount > 1, nil
}

func (p *NodeObserver) isMasterGroup() bool {
	return p.NodeGroup == MasterNgName
}
