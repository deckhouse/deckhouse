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

package usecase

//go:generate minimock -i WatchDog -o ./mock/watchdog_mock.go -g
//go:generate minimock -i NodeWatcher -o ./mock/nodewatcher_mock.go -g
//go:generate minimock -i Decider -o ./mock/decider_mock.go -g
//go:generate minimock -i FallbackDecider -o ./mock/fallbackdecider_mock.go -g
//go:generate minimock -i MemberlistProvider -o ./mock/memberlistprovider_mock.go -g

import (
	"context"
	"sync"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"fencing-agent/internal/lib/backoff"
	"fencing-agent/internal/lib/logger/sl"
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

type MemberlistProvider interface {
	NumMembers() int
}

type HealthMonitor struct {
	mu         *sync.Mutex
	logger     *log.Logger
	membership MemberlistProvider
	watchdog   WatchDog
	decider    Decider
	fallback   FallbackDecider
	watcher    NodeWatcher
}

func NewHealthMonitor(
	watcher NodeWatcher,
	membership MemberlistProvider,
	watchdog WatchDog, decider Decider,
	fallbacker FallbackDecider,
	logger *log.Logger) *HealthMonitor {
	return &HealthMonitor{
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
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.watchdog.IsArmed() {
		if err := h.stopWatchdog(); err != nil {
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

			err := h.stopWatchdog()
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
	wrapped := backoff.Wrap(ctx, h.logger, 5, "watchdog", h.watchdog.Start)

	err := wrapped()
	if err != nil {
		return err
	}

	return nil
}

func (h *HealthMonitor) stopWatchdog() error {
	return h.watchdog.Stop()
}
