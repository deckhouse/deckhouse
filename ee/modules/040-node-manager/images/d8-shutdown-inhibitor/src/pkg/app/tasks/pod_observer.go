/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"
	"d8_shutdown_inhibitor/pkg/system"
)

// PodObserver starts to check Pods on node and stops inhibitors when no pods to wait remain.
type PodObserver struct {
	NodeName              string
	PodsCheckingInterval  time.Duration
	WallBroadcastInterval time.Duration
	PodMatchers           []kubernetes.PodMatcher
	ShutdownSignalCh      <-chan struct{}
	StartCordonCh         chan<- struct{}
	StopInhibitorsCh      chan<- struct{}
	stopOnce             sync.Once
}

func (p *PodObserver) Name() string {
	return "podObserver"
}

const wallMessage = `Pods with shutdown inhibitor label are still running, waiting for them to stop.
Use 'kubectl get po -A -l pod.deckhouse.io/inhibit-node-shutdown' to list them or
use 'kubectl drain' to move Pods to other Nodes.
`

func (p *PodObserver) Run(ctx context.Context, errCh chan error) {
	// Stage 1. Wait for shutdown.
	fmt.Printf("podObserver: wait for PrepareForShutdown signal or power key press\n")
	select {
	case <-ctx.Done():
		fmt.Printf("podObserver(s1): stop on context cancel\n")
		return
	case <-p.ShutdownSignalCh:
		fmt.Printf("podObserver(s1): catch prepare shutdown signal, start pods checker\n")
	}

	// Stage 2. Wait for Pods to stop.
	ticker := time.NewTicker(p.PodsCheckingInterval)
	defer ticker.Stop()

	lastWall := time.Time{}
	for {
		// Check for global stop (do it at the beginning so 'continue' work correctly).
		select {
		case <-ctx.Done():
			fmt.Printf("podObserver(s2): stop on context cancel\n")
			return
		default:
		}
		matchedPods, err := p.ListMatchedPods()
		if err != nil {
			fmt.Printf("podObserver(s2): list matched Pods: %v\n", err)
			if ee, ok := err.(*exec.ExitError); ok {
				fmt.Printf("   stderr: %v\n", string(ee.Stderr))
			}
		} else {
			if len(matchedPods) == 0 {
				fmt.Printf("podObserver(s2): no pods to wait, unlock inhibitors and exit\n")
				err = nodecondition.GracefulShutdownPostpone().UnsetOnUnlock(p.NodeName)
				if err != nil {
					fmt.Printf("podObserver(s2): update Node condition: %v\n", err)
				}
				close(p.StopInhibitorsCh)
				return
			}

			p.stopOnce.Do(func() {
				fmt.Printf("podObserver(s2): %d pods are still running, triggering node cordon\n", len(matchedPods))
				close(p.StartCordonCh)
			})
			fmt.Printf("podObserver(s2): %d pods are still running\n", len(matchedPods))

			err = nodecondition.GracefulShutdownPostpone().SetPodsArePresent(p.NodeName)
			if err != nil {
				// Will retry on next iteration, just log the error.
				fmt.Printf("podObserver(s2): update Node condition: %v\n", err)
			}

			// Reduce wall broadcast messages with longer interval than pods checking interval.
			now := time.Now()
			if lastWall.IsZero() || lastWall.Add(p.WallBroadcastInterval).Before(now) {
				err = system.WallMessage(wallMessage)
				if err != nil {
					// Will retry on next iteration, just log the error.
					fmt.Printf("podObserver(s2): error sending broadcast message: %v\n", err)
				}
				lastWall = now
			}
		}

		// Wait for ticker.
		<-ticker.C
	}
}

func (p *PodObserver) ListMatchedPods() ([]kubernetes.Pod, error) {
	if len(p.PodMatchers) == 0 {
		return nil, nil
	}

	kubectl := kubernetes.NewDefaultKubectl()
	podList, err := kubectl.ListPods(p.NodeName)
	if err != nil {
		return nil, err
	}

	if podList == nil || len(podList.Items) == 0 {
		return nil, nil
	}

	matchedPods := kubernetes.FilterPods(podList.Items, p.PodMatchers...)

	return matchedPods, nil
}
