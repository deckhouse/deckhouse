/*
Copyright 2026 Flant JSC

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

package agent

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/adapters/events"
	"fencing-agent/internal/adapters/kubeclient"
	"fencing-agent/internal/adapters/memberlist"
	watchdogdevice "fencing-agent/internal/adapters/watchdog"
	"fencing-agent/internal/config"
	"fencing-agent/internal/controllers/health"
	"fencing-agent/internal/domain"
	"fencing-agent/internal/usecase/join"
	"fencing-agent/internal/usecase/membership"
	"fencing-agent/internal/usecase/watchdog"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
)

type Agent struct {
	cfg      *config.Config
	deps     Deps
	identity domain.NodeIdentity
	sla      v1alpha1.FencingSLAProfileSpec
	logger   *log.Logger
}

func New(cfg *config.Config, deps Deps, identity domain.NodeIdentity, sla v1alpha1.FencingSLAProfileSpec, logger *log.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		deps:     deps,
		identity: identity,
		sla:      sla,
		logger:   logger,
	}
}

func (a *Agent) memberlistConfig() memberlist.Config {
	return memberlist.Config{
		NodeName:      a.identity.Name,
		NodeGroup:     a.cfg.NodeGroup,
		AdvertiseAddr: a.identity.IP,
		Port:          a.cfg.MemberlistPort,
		Tuning:        a.sla.Memberlist,
	}
}

func (a *Agent) watchdogParams() watchdog.Params {
	return watchdog.Params{
		FeedInterval: a.sla.Watchdog.FeedInterval.Duration,
		Timeout:      a.sla.Watchdog.Timeout.Duration,
	}
}

func (a *Agent) joinParams() join.Params {
	return join.Params{
		NodeName:         a.identity.Name,
		NodeIP:           a.identity.IP,
		NodeGroup:        a.cfg.NodeGroup,
		MemberlistPort:   a.cfg.MemberlistPort,
		APITimeout:       a.sla.Fallback.KubernetesAPITimeout.Duration,
		RetryInterval:    a.sla.Rejoin.Interval.Duration,
		MaxRetryInterval: a.sla.Rejoin.MaxInterval.Duration,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	if a.deps.K8sClient == nil || a.deps.FencingClient == nil {
		return errors.New("agent dependencies are not wired: K8sClient and FencingClient are required")
	}

	a.logger.Info("fencing-agent starting",
		"node", a.identity.Name,
		"node_uid", a.identity.UID,
		"node_ip", a.identity.IP,
		"node_group", a.cfg.NodeGroup,
		"profile", a.cfg.ProfileRefName,
		"probe_interval", a.sla.Memberlist.ProbeInterval.Duration.String(),
		"memberlist_port", a.cfg.MemberlistPort,
		"watchdog_device", a.cfg.WatchdogDevice,
		"watchdog_feed_interval", a.sla.Watchdog.FeedInterval.Duration.String(),
		"watchdog_timeout", a.sla.Watchdog.Timeout.Duration.String(),
		"api_socket_path", a.cfg.APISocketPath,
	)

	cluster, err := memberlist.New(a.memberlistConfig(), a.logger)
	if err != nil {
		return fmt.Errorf("create gossip network: %w", err)
	}

	defer func() {
		if shutdownErr := cluster.Shutdown(); shutdownErr != nil {
			a.logger.Error("shutdown gossip network", "error", shutdownErr)
		}
	}()

	recorder := events.New(a.deps.K8sClient, a.identity, a.logger)
	defer recorder.Shutdown()

	// Expected membership: every Node labeled into the NodeGroup, served from
	// the informer cache. The join seed list and the quorum size come from the
	// same view, so they can never diverge.
	members := membership.New(a.logger)

	watcher, err := kubeclient.NewNodeWatcher(a.deps.K8sClient, a.cfg.NodeGroup, members, a.logger)
	if err != nil {
		return fmt.Errorf("create node watcher: %w", err)
	}

	// The own Node is watched by its own informer: the watchdog must keep seeing
	// maintenance annotations even if the Node loses the NodeGroup label.
	selfState := watchdog.NewSelfState(a.identity.UID, a.logger)

	selfWatcher, err := kubeclient.NewSelfWatcher(a.deps.K8sClient, a.identity.Name, selfState, a.logger)
	if err != nil {
		return fmt.Errorf("create own node watcher: %w", err)
	}

	watchdogManager := watchdog.New(a.watchdogParams(), watchdog.Deps{
		Open: func() (watchdog.Device, error) {
			return watchdogdevice.Open(a.cfg.WatchdogDevice, a.logger)
		},
		Nowayout: func() (bool, error) {
			return watchdogdevice.Nowayout(a.cfg.WatchdogDevice)
		},
		State:      selfState,
		Events:     recorder,
		ShouldFeed: feedGate,
	}, a.logger)

	defer watchdogManager.Close()

	joiner := join.New(members, cluster, a.joinParams(), a.logger)

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		ready := func() bool { return joiner.Joined() && watchdogManager.Ready() }

		return health.NewServer(a.cfg.HealthProbeBindAddress, a.logger, ready, watchdogManager.Alive).Run(gctx)
	})

	g.Go(func() error {
		watcher.Run(gctx)

		return nil
	})

	g.Go(func() error {
		selfWatcher.Run(gctx)

		return nil
	})

	g.Go(func() error {
		// WaitForCacheSync blocks while the API is unreachable: the pod simply
		// stays NotReady until it recovers. It returns false only on shutdown,
		// never as a verdict on the cluster state (the profile, in contrast,
		// fails closed).
		a.logger.Info("waiting for node cache sync")

		if !watcher.WaitForSync(gctx) {
			a.logger.Info("node cache sync aborted by shutdown")

			return nil
		}

		members.MarkSynced()

		if !selfWatcher.WaitForSync(gctx) {
			a.logger.Info("own node cache sync aborted by shutdown")

			return nil
		}

		joiner.Bootstrap(gctx)

		if gctx.Err() != nil {
			return nil
		}

		// The watchdog is armed only here: an armed device promises that this agent
		// can see its NodeGroup, and a watchdog the agent cannot rely on stops it.
		a.logger.Info("gossip network joined, starting the watchdog")

		return watchdogManager.Run(gctx)
	})

	if err := g.Wait(); err != nil {
		return err
	}

	a.logger.Info("fencing-agent stopped")

	return nil
}

func feedGate() (bool, string) {
	return true, "quorum gate is not implemented yet"
}
