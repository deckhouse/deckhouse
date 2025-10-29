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

type PowerKeyInhibitor struct {
	UnlockInhibitorsCh <-chan struct{}

	dbusCon     *systemd.DBusCon
	inhibitLock systemd.InhibitLock
}

func (p *PowerKeyInhibitor) Name() string {
	return "powerKeyInhibitor"
}

func (p *PowerKeyInhibitor) Run(ctx context.Context, errCh chan error) {
	err := p.acquireLock()
	if err != nil {
		errCh <- fmt.Errorf("powerKeyInhibitor acquireLock: %w", err)
		return
	}

	if p.inhibitLock == 0 {
		errCh <- fmt.Errorf("powerKeyInhibitor: lock not acquired")
		return
	}
	dlog.Info("power key inhibitor: lock acquired")

	select {
	case <-ctx.Done():
		dlog.Info("power key inhibitor: unlock on context cancel")
	case <-p.UnlockInhibitorsCh:
		dlog.Info("power key inhibitor: unlock on shutdown requirements met")
	}

	err = p.dbusCon.ReleaseInhibitLock(p.inhibitLock)
	if err != nil {
		dlog.Error("power key inhibitor: unlock error", dlog.Err(err), slog.Int("lock", int(p.inhibitLock)))
		return
	}
	dlog.Info("power key inhibitor: lock released")
}

func (p *PowerKeyInhibitor) acquireLock() error {
	systemBus, err := systemd.NewDBusCon()
	if err != nil {
		return fmt.Errorf("initiate DBus connection: %v", err)
	}
	p.dbusCon = systemBus

	lock, err := p.dbusCon.InhibitPowerKey()
	if err != nil {
		return err
	}
	if p.inhibitLock != 0 {
		p.dbusCon.ReleaseInhibitLock(p.inhibitLock)
	}
	p.inhibitLock = lock
	return nil
}
