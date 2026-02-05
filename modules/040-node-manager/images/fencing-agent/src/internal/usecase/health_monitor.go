package usecase

//go:generate minimock -i WatchDog -o ./mock/watchdog_mock.go -g
//go:generate minimock -i NodeWatcher -o ./mock/nodewatcher_mock.go -g
//go:generate minimock -i Decider -o ./mock/decider_mock.go -g
//go:generate minimock -i FallbackDecider -o ./mock/fallbackdecider_mock.go -g
//go:generate minimock -i ClusterProvider -o ./mock/clusterprovider_mock.go -g
//go:generate minimock -i MemberlistProvider -o ./mock/memberlistprovider_mock.go -g

import (
	"context"
	"fencing-agent/internal/helper/logger/sl"
	"fmt"
	"log/slog"
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

type NodeWatcher interface {
	IsMaintenanceMode() bool
}

type Decider interface {
	ShouldFeed(memberNum int) bool
}

type FallbackDecider interface {
	ShouldFeed(ctx context.Context) bool
}

type ClusterProvider interface {
	SetNodeLabel(ctx context.Context, key string, value string) error
	RemoveNodeLabel(ctx context.Context, key string) error
}

type MemberlistProvider interface {
	NumMembers() int
}

type HealthMonitor struct {
	mu         *sync.Mutex
	logger     *log.Logger
	cluster    ClusterProvider
	membership MemberlistProvider
	watchdog   WatchDog
	decider    Decider
	fallback   FallbackDecider
	watcher    NodeWatcher
}

func NewHealthMonitor(
	watcher NodeWatcher,
	cluster ClusterProvider,
	membership MemberlistProvider,
	watchdog WatchDog, decider Decider,
	fallbacker FallbackDecider,
	logger *log.Logger) *HealthMonitor {
	return &HealthMonitor{
		cluster:    cluster,
		membership: membership,
		watchdog:   watchdog,
		mu:         &sync.Mutex{},
		logger:     logger,
		decider:    decider,
		fallback:   fallbacker,
		watcher:    watcher,
	}
}

func (h *HealthMonitor) Start(ctx context.Context, watchdogTimeout int) error {
	timeout := time.Duration(watchdogTimeout/2-1) * time.Second
	if err := h.startWatchdogBackoff(ctx); err != nil {
		return err
	}
	go func() {
		ticker := time.NewTicker(timeout)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.check(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (h *HealthMonitor) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.watchdog.IsArmed() {
		if err := h.stopWatchdog(ctx); err != nil {
			h.logger.Error("unable to stop watchdog", sl.Err(err))
		}
	}
	h.logger.Info("health monitor stopped successfully")
}

func (h *HealthMonitor) check(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.watcher.IsMaintenanceMode() {

		h.logger.Info("node is in maintenance mode")

		if h.watchdog.IsArmed() {

			h.logger.Info("node is in maintenance mode, watchdog is armed, disarming watchdog")

			err := h.stopWatchdog(ctx)
			if err == nil {
				h.logger.Info("watchdog disarmed successfully")
				return
			}

			h.logger.Error("unable to disarm watchdog, continue feeding", sl.Err(err))

			if feedErr := h.watchdog.Feed(); feedErr != nil {
				h.logger.Error("unable to feed watchdog", sl.Err(feedErr))
			}
			return
		}
		return
	}

	if !h.watchdog.IsArmed() {
		h.logger.Info("watchdog is not armed, arming watchdog")
		if err := h.startWatchdogBackoff(ctx); err != nil {
			h.logger.Error("unable to arm watchdog", sl.Err(err))
			return
		}

		h.logger.Info("watchdog armed successfully")
	}

	numMembers := h.membership.NumMembers()

	if h.decider.ShouldFeed(numMembers) {
		if feedErr := h.watchdog.Feed(); feedErr != nil {
			h.logger.Error("unable to feed watchdog", sl.Err(feedErr))
		}
		h.logger.Info("quorum feeding")
		return
	}
	if h.fallback.ShouldFeed(ctx) {
		if feedErr := h.watchdog.Feed(); feedErr != nil {
			h.logger.Error("unable to feed watchdog", sl.Err(feedErr))
		}
		h.logger.Info("fallback feeding")
		return
	}
}

func (h *HealthMonitor) startWatchdogBackoff(ctx context.Context) error {
	const triesLimit = 5
	currenTry := 1

	err := h.watchdog.Start()

	base, mx := time.Second, time.Minute

	for backoff := base; err != nil; backoff <<= 1 {
		if currenTry > triesLimit {
			return fmt.Errorf("failed to start watchdog, tries limit reached: %w", err)
		}

		if backoff > mx {
			backoff = mx
		}
		h.logger.Warn("failed to start watchdog", sl.Err(err), slog.String("backoff", backoff.String()), slog.Int("tries", currenTry))

		time.Sleep(backoff)

		err = h.watchdog.Start()

		currenTry++
	}

	// Set node label to indicate fencing is enabled
	if labelErr := h.cluster.SetNodeLabel(ctx, fencingNodeLabel, ""); labelErr != nil {
		h.logger.Error("unable to set node label, disarming watchdog for safety", sl.Err(labelErr))

		if stopErr := h.watchdog.Stop(); stopErr != nil {
			h.logger.Error("failed to stop watchdog after label error", sl.Err(stopErr))
		}

		return labelErr
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
