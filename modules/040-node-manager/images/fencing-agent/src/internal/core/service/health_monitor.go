package service

import (
	"context"
	"fencing-controller/internal/core/ports"
	"time"

	"go.uber.org/zap"
)

// for correct working memberlist must be started before health monitor

type HealthMonitor struct {
	cluster    ports.ClusterProvider
	membership ports.MembershipProvider
	watchdog   ports.WatchDog
	logger     *zap.Logger
}

func NewHealthMonitor(cluster ports.ClusterProvider, membership ports.MembershipProvider, watchdog ports.WatchDog, logger *zap.Logger) *HealthMonitor {
	return &HealthMonitor{
		cluster:    cluster,
		membership: membership,
		watchdog:   watchdog,
		logger:     logger,
	}
}

func (h *HealthMonitor) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval) // TODO mb without interval
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			h.check(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (h *HealthMonitor) check(ctx context.Context) {
	inMaintenance, _ := h.cluster.IsMaintenanceMode(ctx)
	if inMaintenance {
		if h.watchdog.IsArmed() {
			if err := h.watchdog.Stop(); err != nil {
				h.logger.Error("Unable to disarm watchdog", zap.Error(err))
			}
		}
		return // TODO logging
	}
	if !h.watchdog.IsArmed() {
		if err := h.watchdog.Start(); err != nil {
			h.logger.Error("Unable to arm watchdog", zap.Error(err))
		}
	}
	shouldFeed := h.shouldFeedWatchDog(ctx)
	if shouldFeed {
		if err := h.watchdog.Feed(); err != nil {
			h.logger.Error("Unable to feed watchdog", zap.Error(err))
		}
	} else {
		h.logger.Debug("Not feeding the watchdog, will reboot soon")
	}
}
func (h *HealthMonitor) shouldFeedWatchDog(ctx context.Context) bool {
	if h.cluster.IsAvailable(ctx) {
		return true
	}
	if h.membership.NumOtherMembers() > 0 || h.membership.IsAlone() {
		return true
	}
	return false
}
