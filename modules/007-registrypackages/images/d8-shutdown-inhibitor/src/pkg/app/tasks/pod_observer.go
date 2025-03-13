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
	"time"

	"d8_shutdown_inhibitor/pkg/app/containerd"
	"d8_shutdown_inhibitor/pkg/system"
)

// PodObserver starts to check Pods on node and stops inhibitors when no pods to wait remain.
type PodObserver struct {
	PodsCheckingInterval  time.Duration
	WallBroadcastInterval time.Duration
	PodMatchers           []containerd.PodMatcher
	ShutdownSignalCh      <-chan struct{}
	//PowerKeyPressedCh <-chan struct{}
	StopInhibitorsCh chan<- struct{}
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
	case <-p.ShutdownSignalCh:
		fmt.Printf("podObserver(s1): catch prepare shutdown signal, start pods checker\n")
		//case <-p.PowerKeyPressedCh:
		//	fmt.Printf("podObserver(s1): catch power key press, start pods checker\n")
	}

	// Stage 2. Wait for Pods to stop.
	ticker := time.NewTicker(p.PodsCheckingInterval)
	defer ticker.Stop()

	lastWall := time.Time{}
	for {
		matchedPods, err := p.podsToWait(ctx)
		if err != nil {
			fmt.Printf("podObserver(s2): error listing Pods: %v\n", err)
			// TODO add maximum retry count.
			continue
		}
		if matchedPods == 0 {
			fmt.Printf("podObserver(s2): no pods to wait, unlock inhibitors and exit\n")
			close(p.StopInhibitorsCh)
			return
		}
		fmt.Printf("podObserver(s2): %d pods are still running\n", matchedPods)

		// Limit wall broadcast messages with longer interval than pods checking interval.
		now := time.Now()
		if lastWall.IsZero() || lastWall.Add(p.WallBroadcastInterval).Before(now) {
			err = system.WallMessage(wallMessage)
			if err != nil {
				fmt.Printf("podObserver(s2): error sending broadcast message: %v\n", err)
			}
			lastWall = now
		}

		// Wait for ticker or global stop.
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *PodObserver) podsToWait(ctx context.Context) (int, error) {
	if len(p.PodMatchers) == 0 {
		return 0, nil
	}

	podList, err := p.listPods(ctx)
	if err != nil {
		// Assume we still have pods on error. Caller will retry listing.
		return 0, err
	}

	if podList == nil || len(podList.Items) == 0 {
		return 0, nil
	}

	matchedPods := containerd.FilterPods(podList.Items, p.PodMatchers...) // := 0
	//for _, pod := range podList.Items {
	//	if p.PodMatcher(pod) {
	//		matched++
	//	}
	//}

	return len(matchedPods), nil
}

func (p *PodObserver) listPods(ctx context.Context) (*containerd.PodList, error) {
	// Run crictl pods
	podList, err := containerd.ListPods(ctx)
	if err != nil {
		fmt.Printf("list pods: %v\n", err)
		return nil, err
	}

	return podList, nil
}
