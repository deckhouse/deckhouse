/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"d8_shutdown_inhibitor/pkg/app/nodecondition"
	"d8_shutdown_inhibitor/pkg/kubernetes"
	"d8_shutdown_inhibitor/pkg/system"

	corev1 "k8s.io/api/core/v1"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
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
	stopOnce              sync.Once
	Klient                *kubernetes.Klient
	CordonEnabled         bool
}

func (p *PodObserver) Name() string {
	return "podObserver"
}

func (p *PodObserver) getWallMessage() string {
    if p.CordonEnabled {
        return `Pods with shutdown inhibitor label are still running, waiting for them to stop.
Use 'kubectl get po -A -l pod.deckhouse.io/inhibit-node-shutdown' to list them or
use 'kubectl drain' to move Pods to other Nodes.
`
    }
    return `Pods with shutdown inhibitor label are still running, waiting for them to stop.
Use 'kubectl get po -A -l pod.deckhouse.io/inhibit-node-shutdown' to list them.
Please terminate these pods gracefully before proceeding with node shutdown.
`
}

func (p *PodObserver) Run(ctx context.Context, errCh chan error) {
	// Stage 1. Wait for shutdown.
	dlog.Info("pod observer: waiting for shutdown signal", slog.String("node", p.NodeName))
	select {
	case <-ctx.Done():
		dlog.Info("pod observer: context cancelled during wait", slog.String("node", p.NodeName))
		return
	case <-p.ShutdownSignalCh:
		dlog.Info("pod observer: received shutdown signal, start checking pods", slog.String("node", p.NodeName))
	}

	// Stage 2. Wait for Pods to stop.
	ticker := time.NewTicker(p.PodsCheckingInterval)
	defer ticker.Stop()

	lastWall := time.Time{}
	for {
		// Check for global stop (do it at the beginning so 'continue' work correctly).
		select {
		case <-ctx.Done():
			dlog.Info("pod observer: context cancelled while monitoring pods", slog.String("node", p.NodeName))
			return
		default:
		}
		matchedPods, err := p.ListMatchedPods(ctx)
		if err != nil {
			dlog.Error("pod observer: list matched pods failed", slog.String("node", p.NodeName), dlog.Err(err))
		} else {
			if len(matchedPods) == 0 {
				dlog.Info("pod observer: no pods with inhibitor label remaining, unlocking inhibitors", slog.String("node", p.NodeName))
				err = nodecondition.GracefulShutdownPostpone(p.Klient).UnsetOnUnlock(ctx, p.NodeName)
				if err != nil {
					dlog.Warn("pod observer: failed to unset node condition", slog.String("node", p.NodeName), dlog.Err(err))
				}
				close(p.StopInhibitorsCh)
				return
			}

			p.stopOnce.Do(func() {
				dlog.Info("pod observer: pods still running, triggering node cordon",
					slog.String("node", p.NodeName),
					slog.Int("pods", len(matchedPods)),
				)
				close(p.StartCordonCh)
			})
			dlog.Info("pod observer: pods still running", slog.String("node", p.NodeName), slog.Int("pods", len(matchedPods)))

			err = nodecondition.GracefulShutdownPostpone(p.Klient).SetPodsArePresent(ctx, p.NodeName)
			if err != nil {
				// Will retry on next iteration, just log the error.
				dlog.Warn("pod observer: failed to update node condition", slog.String("node", p.NodeName), dlog.Err(err))
			}

			// Reduce wall broadcast messages with longer interval than pods checking interval.
			now := time.Now()
			if lastWall.IsZero() || lastWall.Add(p.WallBroadcastInterval).Before(now) {
				err = system.WallMessage(p.getWallMessage())
				if err != nil {
					// Will retry on next iteration, just log the error.
					dlog.Warn("pod observer: failed to send wall message", slog.String("node", p.NodeName), dlog.Err(err))
				}
				lastWall = now
			}
		}

		// Wait for ticker.
		<-ticker.C
	}
}

func (p *PodObserver) ListMatchedPods(ctx context.Context) ([]corev1.Pod, error) {
	if len(p.PodMatchers) == 0 {
		return nil, nil
	}

	if p.Klient == nil {
		return nil, fmt.Errorf("kube client is not initialized")
	}

	podList, err := p.Klient.ListPodsOnNode(ctx, p.NodeName)
	if err != nil {
		return nil, err
	}

	if podList == nil || len(podList.Items) == 0 {
		return nil, nil
	}

	filtered := p.Klient.FilterPods(podList, p.PodMatchers...)
	if len(filtered) == 0 {
		return nil, nil
	}

	return filtered, nil
}
