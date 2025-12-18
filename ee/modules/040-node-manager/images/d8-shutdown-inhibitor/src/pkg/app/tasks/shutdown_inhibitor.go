/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/systemd"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

type ShutdownInhibitor struct {
	UnlockCtx        context.Context
	ShutdownSignalCh chan<- struct{}

	dbusCon     *systemd.DBusCon
	inhibitLock systemd.InhibitLock
}

func (s *ShutdownInhibitor) Name() string {
	return "shutdownInhibitor"
}

func (s *ShutdownInhibitor) Run(ctx context.Context, errCh chan error) {
	err := s.acquireLock()
	if err != nil {
		errCh <- fmt.Errorf("shutdownInhibitor acquire lock: %w", err)
		return
	}
	if s.inhibitLock == 0 {
		errCh <- fmt.Errorf("shutdownInhibitor: lock not acquired")
		return
	}
	dlog.Info("shutdown inhibitor: delay lock acquired")

	shutdownSignalCh := s.waitForShutdownSignal()

	// Stage 1: wait for shutdown signal.
	select {
	case <-ctx.Done():
		dlog.Info("shutdown inhibitor: unlock on context cancel")
		return
	case <-shutdownSignalCh:
		dlog.Info("shutdown inhibitor: received shutdown signal, triggering pod checker")
		close(s.ShutdownSignalCh)
	}

	// Stage 2: wait for shutdown requirements.
	select {
	case <-ctx.Done():
		dlog.Info("shutdown inhibitor: unlock on context cancel (stage2)")
	case <-s.UnlockCtx.Done():
		dlog.Info("shutdown inhibitor: shutdown requirements met, unlocking")
	}

	err = s.dbusCon.ReleaseInhibitLock(s.inhibitLock)
	if err != nil {
		dlog.Error("shutdown inhibitor: unlock error", slog.Int("lock", int(s.inhibitLock)), dlog.Err(err))
		return
	}
	dlog.Info("shutdown inhibitor: lock released")
}

func (s *ShutdownInhibitor) acquireLock() error {
	systemBus, err := systemd.NewDBusCon()
	if err != nil {
		return fmt.Errorf("initiate DBus connection: %v", err)
	}
	s.dbusCon = systemBus

	lock, err := s.dbusCon.InhibitShutdown()
	if err != nil {
		return err
	}
	if s.inhibitLock != 0 {
		s.dbusCon.ReleaseInhibitLock(s.inhibitLock)
	}
	s.inhibitLock = lock
	return nil
}

func (s *ShutdownInhibitor) waitForShutdownSignal() <-chan bool {
	ch, err := s.dbusCon.MonitorShutdown()
	if err != nil {
		dlog.Error("shutdown inhibitor: failed to monitor shutdown signal", dlog.Err(err))
		return nil
	}
	return ch
}
