/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tasks

import (
	"context"
	"fmt"
	"os"
)

const (
	reportDir         = "/var/run/d8-shutdown-inhibitor"
	enabledFilePath   = "/var/run/d8-shutdown-inhibitor/enabled"
	inhibitedFilePath = "/var/run/d8-shutdown-inhibitor/inhibited"
)

// StatusReporter is a task that reports status for external monitoring,
// e.g. for kubelet.
// TODO "enabled" file should be created via systemd unit configuration.
type StatusReporter struct {
	// UnlockInhibitorsCh is a channel to get event about unlocking inhibitors.
	UnlockInhibitorsCh <-chan struct{}
}

func (s *StatusReporter) Name() string {
	return "statusReporter"
}

func (s *StatusReporter) Run(ctx context.Context, errCh chan error) {
	err := s.ensureReportDir()
	if err != nil {
		errCh <- fmt.Errorf("statusReporter ensure report directory: %w", err)
		return
	}

	// Create enabled file to report that graceful shutdown is enabled.
	err = s.createFiles()
	if err != nil {
		errCh <- fmt.Errorf("statusReporter create files: %w", err)
		return
	}

	// Wait until inhibitors are unlocked.
	select {
	case <-ctx.Done():
		fmt.Printf("statusReporter(s1): stop on global exit\n")
	case <-s.UnlockInhibitorsCh:
		fmt.Printf("statusReporter(s1): inhibitors unlocked, remove file reports\n")
	}

	s.cleanupFiles()
}

func (s *StatusReporter) ensureReportDir() error {
	err := os.Mkdir(reportDir, 0755)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("create report directory: %w", err)
	}
	// Ignore error if directory already exists.
	return nil
}

func (s *StatusReporter) createFiles() error {
	_, err := os.Create(enabledFilePath)
	if err != nil {
		return fmt.Errorf("create enabled file: %w", err)
	}

	_, err = os.Create(inhibitedFilePath)
	if err != nil {
		return fmt.Errorf("create inhibited file: %w", err)
	}

	return nil
}

func (s *StatusReporter) cleanupFiles() {
	_ = os.Remove(enabledFilePath)
	_ = os.Remove(inhibitedFilePath)
}
