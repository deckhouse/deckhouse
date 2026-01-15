package service

import (
	"context"
	"fencing-agent/internal/core/ports"
	"time"

	"go.uber.org/zap"
)

const (
	fencingNodeLabel = "node-manager.deckhouse.io/fencing-enabled"
)

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

func (h *HealthMonitor) Stop(ctx context.Context) error {
	if h.watchdog.IsArmed() {
		return h.stopWatchdog(ctx)
	}
	return nil
}

func (h *HealthMonitor) check(ctx context.Context) {
	inMaintenance, err := h.cluster.IsMaintenanceMode(ctx)
	if err != nil {
		h.logger.Debug("Cannot check maintenance mode", zap.Error(err))
		inMaintenance = false
	}
	if inMaintenance {
		if h.watchdog.IsArmed() {
			if err := h.stopWatchdog(ctx); err != nil {
				h.logger.Error("Unable to disarm watchdog", zap.Error(err))
			}
		}
		h.logger.Info("Maintenance mode is on, so not feeding the watchdog")
		return
	}
	if !h.watchdog.IsArmed() {
		h.logger.Info("Arming watchdog")
		if err := h.startWatchdog(ctx); err != nil {
			h.logger.Error("Unable to arm watchdog", zap.Error(err))
			return
		}
	}
	shouldFeed := h.shouldFeedWatchDog(ctx)
	if shouldFeed {
		h.logger.Debug("Feeding the watchdog")
		if err := h.watchdog.Feed(); err != nil {
			h.logger.Error("Unable to feed watchdog", zap.Error(err))
		}
	} else {
		h.logger.Warn("Not feeding the watchdog, will reboot soon")
	}
}

func (h *HealthMonitor) startWatchdog(ctx context.Context) error {
	if err := h.watchdog.Start(); err != nil {
		return err
	}

	// Set node label to indicate fencing is enabled
	if err := h.cluster.SetNodeLabel(ctx, fencingNodeLabel, ""); err != nil {
		h.logger.Error("Unable to set node label, disarming watchdog for safety", zap.Error(err))
		if stopErr := h.watchdog.Stop(); stopErr != nil {
			h.logger.Error("Failed to stop watchdog after label error", zap.Error(stopErr))
		}
		return err
	}

	return nil
}

func (h *HealthMonitor) stopWatchdog(ctx context.Context) error {
	if err := h.cluster.RemoveNodeLabel(ctx, fencingNodeLabel); err != nil {
		h.logger.Error("Unable to remove node label", zap.Error(err))
	}

	if err := h.watchdog.Stop(); err != nil {
		return err
	}

	return nil
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
