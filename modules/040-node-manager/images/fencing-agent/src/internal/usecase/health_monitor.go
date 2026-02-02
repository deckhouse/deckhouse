package usecase

import (
	"context"
	"fencing-agent/internal/helper/logger/sl"
	"fmt"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	fencingNodeLabel = "node-manager.deckhouse.io/fencing-enabled"
)

type WatchDog interface {
	IsArmed() bool
	Feed() error
	Start() error
	Stop() error
}

type ClusterProvider interface {
	IsMaintenanceMode(ctx context.Context) (bool, error)
	SetNodeLabel(ctx context.Context, key string, value string) error
	RemoveNodeLabel(ctx context.Context, key string) error
}

type FallbackProvider interface {
	Alive(ctx context.Context) bool
}

type MemberlistProvider interface {
	NumMembers() int
	IsAlone() bool
}

type Decider interface {
	HasQuorum(ctx context.Context) bool
}

type HealthMonitor struct {
	cluster    ClusterProvider
	membership MemberlistProvider
	watchdog   WatchDog
	mu         *sync.Mutex
	logger     *log.Logger
	decider    Decider
	fallbacker FallbackProvider
}

func NewHealthMonitor(cluster ClusterProvider, membership MemberlistProvider, watchdog WatchDog, decider Decider, fallbacker FallbackProvider, logger *log.Logger) *HealthMonitor {
	return &HealthMonitor{
		cluster:    cluster,
		membership: membership,
		watchdog:   watchdog,
		mu:         &sync.Mutex{},
		logger:     logger,
		decider:    decider,
		fallbacker: fallbacker,
	}
}

func (h *HealthMonitor) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval) // TODO mb without interval
	defer ticker.Stop()

	// TODO think
	if err := h.startWatchdog(ctx); err != nil {
		panic(err)
	}

	for {
		select {
		case <-ticker.C:
			h.check(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *HealthMonitor) Stop(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.watchdog.IsArmed() {
		if err := h.stopWatchdog(ctx); err != nil {
			return fmt.Errorf("unable to disarm watchdog: %w", err)
		}
	}
	h.logger.Info("health monitor stopped successfully")
	return nil
}

func (h *HealthMonitor) check(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if gotQuorum := h.decider.HasQuorum(ctx); !gotQuorum {
		if alive := h.fallbacker.Alive(ctx); !alive {
			h.logger.Error("no quorum, restart soon...")
			return
		}
	}
	if err := h.watchdog.Feed(); err != nil {
		h.logger.Error("failed to feed watchdog, restart soon...", err)
		return
	}
	h.logger.Info("got quorum, feed watchdog")
}

func (h *HealthMonitor) startWatchdog(ctx context.Context) error {
	if err := h.watchdog.Start(); err != nil {
		return err
	}

	// Set node label to indicate fencing is enabled
	if err := h.cluster.SetNodeLabel(ctx, fencingNodeLabel, ""); err != nil {
		h.logger.Error("unable to set node label, disarming watchdog for safety", sl.Err(err))
		if stopErr := h.watchdog.Stop(); stopErr != nil {
			h.logger.Error("failed to stop watchdog after label error", sl.Err(stopErr))
		}
		return err
	}

	return nil
}

func (h *HealthMonitor) stopWatchdog(ctx context.Context) error {
	if err := h.cluster.RemoveNodeLabel(ctx, fencingNodeLabel); err != nil {
		h.logger.Error("Unable to remove node label", sl.Err(err))
	}

	if err := h.watchdog.Stop(); err != nil {
		return err
	}

	return nil
}
