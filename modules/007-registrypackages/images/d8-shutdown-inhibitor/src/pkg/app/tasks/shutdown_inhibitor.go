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

type ShutdownInhibitor struct {
	UnlockInhibitorsCh <-chan struct{}
	ShutdownSignalCh   chan<- struct{}
	dbusCon            *systemd.DBusCon
	inhibitLock        systemd.InhibitLock
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
	fmt.Printf("shutdownInhibitor: got delay lock\n")

	shutdownSignalCh := s.waitForShutdownSignal()

	// Stage 1: wait for shutdown signal.
	select {
	case <-ctx.Done():
		fmt.Printf("shutdownInhibitor(s1): unlock on global exit\n")
		return
	case <-shutdownSignalCh:
		fmt.Printf("shutdownInhibitor(s1): Got PrepareShutdownSignal, trigger pod checker\n")
		close(s.ShutdownSignalCh)
	}

	// Stage 2: wait for shutdown requirements.
	select {
	case <-ctx.Done():
		fmt.Printf("shutdownInhibitor(s2): unlock on global exit\n")
	case <-s.UnlockInhibitorsCh:
		fmt.Printf("shutdownInhibitor(s2): unlock on meeting shutdown requirements.\n")
	}

	err = s.dbusCon.ReleaseInhibitLock(s.inhibitLock)
	if err != nil {
		fmt.Printf("shutdownInhibitor: unlock error: %v\n", err)
		return
	}
	fmt.Printf("shutdownInhibitor: unlocked\n")
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
		fmt.Printf("shutdownInhibitor: failed to monitor shutdown signal: %v\n", err)
		return nil
	}
	return ch
}
