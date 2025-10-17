// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func TestNewActionIniter(t *testing.T) {
	initer := newActionIniter()
	require.NotNil(t, initer)
	require.Empty(t, initer.logFile)
}

func TestActionIniter_InitDirectories(t *testing.T) {
	initer := newActionIniter()

	tests := []struct {
		name        string
		dirs        directoriesToInitialize
		expectError bool
		setup       func() string
		cleanup     func(string)
	}{
		{
			name: "create new directory",
			dirs: directoriesToInitialize{},
			setup: func() string {
				tmpDir, err := os.MkdirTemp("", "dhctl-action-test-*")
				require.NoError(t, err)
				testDir := filepath.Join(tmpDir, "new-dir")
				return testDir
			},
			cleanup: func(dir string) {
				os.RemoveAll(filepath.Dir(dir))
			},
			expectError: false,
		},
		{
			name: "directory already exists",
			dirs: directoriesToInitialize{},
			setup: func() string {
				tmpDir, err := os.MkdirTemp("", "dhctl-action-test-*")
				require.NoError(t, err)
				testDir := filepath.Join(tmpDir, "existing-dir")
				err = os.MkdirAll(testDir, 0755)
				require.NoError(t, err)
				return testDir
			},
			cleanup: func(dir string) {
				os.RemoveAll(filepath.Dir(dir))
			},
			expectError: false,
		},
		{
			name: "invalid directory path",
			dirs: directoriesToInitialize{},
			setup: func() string {
				// Try to create directory in non-existent parent with restricted permissions
				return "/root/nonexistent/invalid"
			},
			cleanup:     func(dir string) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := tt.setup()
			defer tt.cleanup(testDir)

			tt.dirs["test dir"] = testDir

			err := initer.initDirectories(tt.dirs)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "Cannot create")
			} else {
				require.NoError(t, err)
				if testDir != "/root/nonexistent/invalid" {
					_, err := os.Stat(testDir)
					require.NoError(t, err, "Directory should exist")
				}
			}
		})
	}
}

func TestActionIniter_InitLogger(t *testing.T) {
	initer := newActionIniter()

	// Create temporary directory for log files
	tmpDir, err := os.MkdirTemp("", "dhctl-action-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Backup original app values
	originalTmpDir := app.TmpDirName
	originalDoNotWrite := app.DoNotWriteDebugLogFile
	originalDebugLogPath := app.DebugLogFilePath

	defer func() {
		app.TmpDirName = originalTmpDir
		app.DoNotWriteDebugLogFile = originalDoNotWrite
		app.DebugLogFilePath = originalDebugLogPath
	}()

	app.TmpDirName = tmpDir

	tests := []struct {
		name               string
		doNotWriteDebugLog bool
		debugLogFilePath   string
		selectedCommand    string
		expectError        bool
		expectLogFile      bool
	}{
		{
			name:               "do not write debug log",
			doNotWriteDebugLog: true,
			debugLogFilePath:   "",
			selectedCommand:    "bootstrap",
			expectError:        false,
			expectLogFile:      false,
		},
		{
			name:               "no selected command",
			doNotWriteDebugLog: false,
			debugLogFilePath:   "",
			selectedCommand:    "",
			expectError:        false,
			expectLogFile:      false,
		},
		{
			name:               "custom debug log path",
			doNotWriteDebugLog: false,
			debugLogFilePath:   filepath.Join(tmpDir, "custom.log"),
			selectedCommand:    "bootstrap",
			expectError:        false,
			expectLogFile:      true,
		},
		{
			name:               "auto-generated log path",
			doNotWriteDebugLog: false,
			debugLogFilePath:   "",
			selectedCommand:    "bootstrap install",
			expectError:        false,
			expectLogFile:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.DoNotWriteDebugLogFile = tt.doNotWriteDebugLog
			app.DebugLogFilePath = tt.debugLogFilePath

			// Create mock kingpin context
			app := kingpin.New("test", "Test application")
			var selectedCmd *kingpin.CmdClause

			if tt.selectedCommand != "" {
				// Create a simple command structure
				if tt.selectedCommand == "bootstrap" {
					selectedCmd = app.Command("bootstrap", "Bootstrap command")
				} else if tt.selectedCommand == "bootstrap install" {
					bootstrapCmd := app.Command("bootstrap", "Bootstrap command")
					selectedCmd = bootstrapCmd.Command("install", "Install command")
				}
			}

			context := &kingpin.ParseContext{
				SelectedCommand: selectedCmd,
			}

			err := initer.initLogger(context)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				if tt.expectLogFile {
					logPath := initer.getLoggerPath()
					require.NotEmpty(t, logPath)

					// Check if log file was created
					_, err := os.Stat(logPath)
					require.NoError(t, err, "Log file should be created")
				} else {
					logPath := initer.getLoggerPath()
					require.Empty(t, logPath)
				}
			}
		})
	}
}

func TestActionIniter_GetLoggerPath(t *testing.T) {
	initer := newActionIniter()

	t.Run("empty log file initially", func(t *testing.T) {
		path := initer.getLoggerPath()
		require.Empty(t, path)
	})

	t.Run("thread safety", func(t *testing.T) {
		// Test concurrent access to getLoggerPath
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func() {
				defer func() { done <- true }()
				path := initer.getLoggerPath()
				_ = path // Use the path to avoid unused variable warning
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestActionIniter_Init(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "dhctl-action-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Backup original app values
	originalTmpDir := app.TmpDirName
	originalDoNotWrite := app.DoNotWriteDebugLogFile

	defer func() {
		app.TmpDirName = originalTmpDir
		app.DoNotWriteDebugLogFile = originalDoNotWrite
	}()

	app.TmpDirName = tmpDir
	app.DoNotWriteDebugLogFile = true // Disable log file creation for simpler testing

	initer := newActionIniter()

	tests := []struct {
		name            string
		selectedCommand string
		expectError     bool
	}{
		{
			name:            "successful initialization",
			selectedCommand: "bootstrap",
			expectError:     false,
		},
		{
			name:            "initialization with _server command",
			selectedCommand: "_server",
			expectError:     false,
		},
		{
			name:            "initialization with no command",
			selectedCommand: "",
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock kingpin context
			app := kingpin.New("test", "Test application")
			var selectedCmd *kingpin.CmdClause

			if tt.selectedCommand != "" {
				selectedCmd = app.Command(tt.selectedCommand, "Test command")
			}

			context := &kingpin.ParseContext{
				SelectedCommand: selectedCmd,
			}

			err := initer.init(context)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify temp directory was created
				_, err := os.Stat(tmpDir)
				require.NoError(t, err, "Temp directory should exist")
			}
		})
	}
}
