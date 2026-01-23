/*
Copyright 2024 Flant JSC

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

package softdog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewWatchdog(t *testing.T) {
	devicePath := "/dev/test-watchdog"

	wd := NewWatchdog(devicePath)

	if wd == nil {
		t.Fatal("Expected NewWatchdog to return non-nil instance")
	}

	if wd.watchdogDeviceName != devicePath {
		t.Errorf("Expected device name '%s', got '%s'", devicePath, wd.watchdogDeviceName)
	}

	if wd.isArmed {
		t.Error("Expected watchdog to be unarmed initially")
	}

	if wd.watchdogDevice != nil {
		t.Error("Expected watchdog device to be nil initially")
	}
}

func TestWatchDog_IsArmed_InitialState(t *testing.T) {
	wd := NewWatchdog("/dev/test")

	if wd.IsArmed() {
		t.Error("Expected IsArmed to return false for new watchdog")
	}
}

func TestWatchDog_Start_Success(t *testing.T) {
	// Create temporary file to simulate watchdog device
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	// Create the file
	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	err := wd.Start()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !wd.IsArmed() {
		t.Error("Expected watchdog to be armed after Start()")
	}

	if wd.watchdogDevice == nil {
		t.Error("Expected watchdog device to be open")
	}

	// Cleanup
	wd.watchdogDevice.Close()
}

func TestWatchDog_Start_DeviceNotFound(t *testing.T) {
	devicePath := "/non/existent/device"

	wd := NewWatchdog(devicePath)

	err := wd.Start()

	if err == nil {
		t.Error("Expected error when device doesn't exist")
	}

	if wd.IsArmed() {
		t.Error("Expected watchdog to remain unarmed when Start fails")
	}

	if !strings.Contains(err.Error(), "Unable to open watchdog device") {
		t.Errorf("Expected error message about unable to open, got: %v", err)
	}
}

func TestWatchDog_Start_AlreadyArmed(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	// First start
	if err := wd.Start(); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Second start should fail
	err := wd.Start()

	if err == nil {
		t.Error("Expected error when starting already armed watchdog")
	}

	if !strings.Contains(err.Error(), "already armed") {
		t.Errorf("Expected 'already armed' error, got: %v", err)
	}

	// Should still be armed
	if !wd.IsArmed() {
		t.Error("Expected watchdog to still be armed")
	}

	// Cleanup
	wd.watchdogDevice.Close()
}

func TestWatchDog_Feed_Success(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	if err := wd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer wd.watchdogDevice.Close()

	err := wd.Feed()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Read the file to verify '1' was written
	content, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Expected data to be written to watchdog device")
	}

	if content[0] != '1' {
		t.Errorf("Expected '1' to be written, got %c", content[0])
	}
}

func TestWatchDog_Feed_NotArmed(t *testing.T) {
	wd := NewWatchdog("/dev/test")

	err := wd.Feed()

	if err == nil {
		t.Error("Expected error when feeding unarmed watchdog")
	}

	if !strings.Contains(err.Error(), "not opened") {
		t.Errorf("Expected 'not opened' error, got: %v", err)
	}
}

func TestWatchDog_Feed_MultipleTimes(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	if err := wd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer wd.watchdogDevice.Close()

	// Feed multiple times
	for i := 0; i < 5; i++ {
		err := wd.Feed()
		if err != nil {
			t.Errorf("Feed #%d failed: %v", i+1, err)
		}
	}

	// Read the file to verify multiple '1's were written
	content, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) != 5 {
		t.Errorf("Expected 5 bytes written, got %d", len(content))
	}

	for i, b := range content {
		if b != '1' {
			t.Errorf("Byte #%d: expected '1', got %c", i, b)
		}
	}
}

func TestWatchDog_Stop_Success(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	if err := wd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	err := wd.Stop()

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if wd.IsArmed() {
		t.Error("Expected watchdog to be unarmed after Stop()")
	}

	// Read the file to verify 'V' (magic close) was written
	content, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Expected magic close byte to be written")
	}

	if content[0] != 'V' {
		t.Errorf("Expected 'V' (magic close) to be written, got %c", content[0])
	}
}

func TestWatchDog_Stop_NotArmed(t *testing.T) {
	wd := NewWatchdog("/dev/test")

	err := wd.Stop()

	if err == nil {
		t.Error("Expected error when stopping unarmed watchdog")
	}

	if !strings.Contains(err.Error(), "already closed") {
		t.Errorf("Expected 'already closed' error, got: %v", err)
	}
}

func TestWatchDog_Stop_ClosedFile(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	if err := wd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Close the file manually to simulate os.ErrClosed
	wd.watchdogDevice.Close()

	err := wd.Stop()

	// Should return nil because of os.ErrClosed handling
	if err != nil {
		t.Errorf("Expected no error when file is already closed, got %v", err)
	}
}

func TestWatchDog_StateTransitions(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	// Initial state: unarmed
	if wd.IsArmed() {
		t.Error("Step 1: Expected unarmed state")
	}

	// Start -> armed
	if err := wd.Start(); err != nil {
		t.Fatalf("Step 2: Start failed: %v", err)
	}

	if !wd.IsArmed() {
		t.Error("Step 2: Expected armed state after Start")
	}

	// Feed -> still armed
	if err := wd.Feed(); err != nil {
		t.Fatalf("Step 3: Feed failed: %v", err)
	}

	if !wd.IsArmed() {
		t.Error("Step 3: Expected armed state after Feed")
	}

	// Stop -> unarmed
	if err := wd.Stop(); err != nil {
		t.Fatalf("Step 4: Stop failed: %v", err)
	}

	if wd.IsArmed() {
		t.Error("Step 4: Expected unarmed state after Stop")
	}
}

func TestWatchDog_StartStopCycle(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	// Multiple start/stop cycles
	for i := 0; i < 3; i++ {
		// Start
		if err := wd.Start(); err != nil {
			t.Fatalf("Cycle %d: Start failed: %v", i+1, err)
		}

		if !wd.IsArmed() {
			t.Errorf("Cycle %d: Expected armed state after Start", i+1)
		}

		// Feed
		if err := wd.Feed(); err != nil {
			t.Fatalf("Cycle %d: Feed failed: %v", i+1, err)
		}

		// Stop
		if err := wd.Stop(); err != nil {
			t.Fatalf("Cycle %d: Stop failed: %v", i+1, err)
		}

		if wd.IsArmed() {
			t.Errorf("Cycle %d: Expected unarmed state after Stop", i+1)
		}
	}
}

func TestWatchDog_Integration_CompleteLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	devicePath := filepath.Join(tmpDir, "watchdog")

	if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	wd := NewWatchdog(devicePath)

	// 1. Initial state
	if wd.IsArmed() {
		t.Error("Expected initial unarmed state")
	}

	// 2. Try to feed before start - should fail
	if err := wd.Feed(); err == nil {
		t.Error("Expected error when feeding before start")
	}

	// 3. Try to stop before start - should fail
	if err := wd.Stop(); err == nil {
		t.Error("Expected error when stopping before start")
	}

	// 4. Start
	if err := wd.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !wd.IsArmed() {
		t.Error("Expected armed state after start")
	}

	// 5. Try to start again - should fail
	if err := wd.Start(); err == nil {
		t.Error("Expected error when starting already armed watchdog")
	}

	// 6. Feed successfully
	if err := wd.Feed(); err != nil {
		t.Errorf("Feed failed: %v", err)
	}

	// 7. Feed again
	if err := wd.Feed(); err != nil {
		t.Errorf("Second feed failed: %v", err)
	}

	// 8. Stop
	if err := wd.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if wd.IsArmed() {
		t.Error("Expected unarmed state after stop")
	}

	// 9. Try to stop again - should fail
	if err := wd.Stop(); err == nil {
		t.Error("Expected error when stopping already closed watchdog")
	}

	// 10. Try to feed after stop - should fail
	if err := wd.Feed(); err == nil {
		t.Error("Expected error when feeding after stop")
	}

	// Verify file contents
	content, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Should have: '1' + '1' + 'V'
	expected := []byte{'1', '1', 'V'}
	if len(content) != len(expected) {
		t.Errorf("Expected %d bytes, got %d", len(expected), len(content))
	}

	for i, b := range expected {
		if i >= len(content) {
			break
		}
		if content[i] != b {
			t.Errorf("Byte %d: expected %c, got %c", i, b, content[i])
		}
	}
}

func TestWatchDog_ErrorMessages(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() *WatchDog
		operation     func(*WatchDog) error
		expectedError string
	}{
		{
			name: "Start when already armed",
			setup: func() *WatchDog {
				tmpDir := t.TempDir()
				devicePath := filepath.Join(tmpDir, "watchdog")
				if err := os.WriteFile(devicePath, []byte{}, 0644); err != nil {
					t.Fatalf("Failed to create file for test: %v", err)
				}
				wd := NewWatchdog(devicePath)
				if err := wd.Start(); err != nil {
					t.Fatalf("Failed to start watchdog: %v", err)
				}
				return wd
			},
			operation:     func(wd *WatchDog) error { return wd.Start() },
			expectedError: "already armed",
		},
		{
			name: "Feed when not armed",
			setup: func() *WatchDog {
				return NewWatchdog("/dev/test")
			},
			operation:     func(wd *WatchDog) error { return wd.Feed() },
			expectedError: "not opened",
		},
		{
			name: "Stop when not armed",
			setup: func() *WatchDog {
				return NewWatchdog("/dev/test")
			},
			operation:     func(wd *WatchDog) error { return wd.Stop() },
			expectedError: "already closed",
		},
		{
			name: "Start with non-existent device",
			setup: func() *WatchDog {
				return NewWatchdog("/non/existent/device")
			},
			operation:     func(wd *WatchDog) error { return wd.Start() },
			expectedError: "Unable to open watchdog device",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd := tt.setup()
			err := tt.operation(wd)

			if err == nil {
				t.Errorf("Expected error containing '%s', got nil", tt.expectedError)
				return
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
			}

			// Cleanup if file was opened
			if wd.watchdogDevice != nil {
				wd.watchdogDevice.Close()
			}
		})
	}
}
