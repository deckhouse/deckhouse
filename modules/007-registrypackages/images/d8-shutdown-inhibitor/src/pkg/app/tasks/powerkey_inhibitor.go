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

	"d8_shutdown_inhibitor/pkg/systemd"
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
	fmt.Printf("powerKeyInhibitor: got lock\n")

	select {
	case <-ctx.Done():
		fmt.Printf("powerKeyInhibitor: unlock on global exit\n")
	case <-p.UnlockInhibitorsCh:
		fmt.Printf("powerKeyInhibitor: unlock on meeting shutdown requirements.\n")
	}

	err = p.dbusCon.ReleaseInhibitLock(p.inhibitLock)
	if err != nil {
		fmt.Printf("powerKeyInhibitor: unlock error: %v\n", err)
		return
	}
	fmt.Printf("powerKeyInhibitor: unlocked\n")
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
