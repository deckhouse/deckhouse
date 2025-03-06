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

	"graceful_shutdown/pkg/systemd"
)

// ShutdownBlockInhibitor additionally block shutdown to prevent immediate reboot.
type ShutdownBlockInhibitor struct {
	UnlockInhibitorsCh <-chan struct{}
	dbusCon            *systemd.DBusCon
	inhibitLock        systemd.InhibitLock
}

func (s *ShutdownBlockInhibitor) Run(ctx context.Context, errCh chan error) {
	err := s.getLock()
	if err != nil {
		errCh <- fmt.Errorf("shutdownBlockInhibitor: get lock: %w", err)
		return
	}
	if s.inhibitLock == 0 {
		errCh <- fmt.Errorf("shutdownBlockInhibitor: lock not acquired")
		return
	}
	fmt.Printf("shutdownBlockInhibitor: lock acquired\n")

	// Wait for shutdown requirements.
	select {
	case <-ctx.Done():
		fmt.Printf("shutdownBlockInhibitor: unlock on global exit\n")
	case <-s.UnlockInhibitorsCh:
		fmt.Printf("shutdownBlockInhibitor: unlock on meeting shutdown requirements.\n")
	}

	err = s.dbusCon.ReleaseInhibitLock(s.inhibitLock)
	if err != nil {
		fmt.Printf("shutdownBlockInhibitor: unlock error: %v\n", err)
		return
	}
	fmt.Printf("shutdownBlockInhibitor: unlocked\n")
}

func (s *ShutdownBlockInhibitor) getLock() error {
	systemBus, err := systemd.NewDBusCon()
	if err != nil {
		return fmt.Errorf("initiate DBus connection: %v", err)
	}
	s.dbusCon = systemBus

	lock, err := s.dbusCon.InhibitShutdownBlock()
	if err != nil {
		return err
	}
	if s.inhibitLock != 0 {
		s.dbusCon.ReleaseInhibitLock(s.inhibitLock)
	}
	s.inhibitLock = lock
	return nil
}
