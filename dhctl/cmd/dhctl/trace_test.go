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
)

func TestEnableTrace(t *testing.T) {
	tests := []struct {
		name              string
		envValue          string
		expectFiles       bool
		expectedTraceFile string
		expectedCPUFile   string
		expectError       bool
	}{
		{
			name:        "trace disabled with empty env",
			envValue:    "",
			expectFiles: false,
			expectError: false,
		},
		{
			name:        "trace disabled with '0'",
			envValue:    "0",
			expectFiles: false,
			expectError: false,
		},
		{
			name:        "trace disabled with 'no'",
			envValue:    "no",
			expectFiles: false,
			expectError: false,
		},
		{
			name:              "trace enabled with '1'",
			envValue:          "1",
			expectFiles:       true,
			expectedTraceFile: "trace.out",
			expectedCPUFile:   "pprof.cpu",
			expectError:       false,
		},
		{
			name:              "trace enabled with 'yes'",
			envValue:          "yes",
			expectFiles:       true,
			expectedTraceFile: "trace.out",
			expectedCPUFile:   "pprof.cpu",
			expectError:       false,
		},
		{
			name:              "trace enabled with custom filename",
			envValue:          "custom_trace",
			expectFiles:       true,
			expectedTraceFile: "custom_trace",
			expectedCPUFile:   "custom_trace.prof.cpu",
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary directory for test files
			tmpDir, err := os.MkdirTemp("", "dhctl-trace-test-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Change to temp directory to avoid cluttering the current directory
			originalDir, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(originalDir)

			err = os.Chdir(tmpDir)
			require.NoError(t, err)

			// Set environment variable
			originalEnv := os.Getenv("DHCTL_TRACE")
			defer os.Setenv("DHCTL_TRACE", originalEnv)

			os.Setenv("DHCTL_TRACE", tt.envValue)

			// Call enableTrace
			shutdownFn, err := enableTrace()

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, shutdownFn)

			if tt.expectFiles {
				// Check that trace files were created
				_, err := os.Stat(tt.expectedTraceFile)
				require.NoError(t, err, "Trace file should be created")

				_, err = os.Stat(tt.expectedCPUFile)
				require.NoError(t, err, "CPU profile file should be created")
			}

			// Call shutdown function to clean up
			shutdownFn()

			if tt.expectFiles {
				// Files should still exist after shutdown
				_, err := os.Stat(tt.expectedTraceFile)
				require.NoError(t, err, "Trace file should exist after shutdown")

				_, err = os.Stat(tt.expectedCPUFile)
				require.NoError(t, err, "CPU profile file should exist after shutdown")
			}
		})
	}
}

func TestEnableTraceFileCreationError(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "dhctl-trace-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// Set environment variable
	originalEnv := os.Getenv("DHCTL_TRACE")
	defer os.Setenv("DHCTL_TRACE", originalEnv)

	t.Run("trace file creation error", func(t *testing.T) {
		// Try to create trace file in non-existent directory
		invalidPath := filepath.Join(tmpDir, "nonexistent", "trace.out")
		os.Setenv("DHCTL_TRACE", invalidPath)

		shutdownFn, err := enableTrace()
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to create trace output file")
		require.NotNil(t, shutdownFn) // Should return a no-op function even on error
	})

	t.Run("cpu profile file creation error", func(t *testing.T) {
		// Create trace file successfully but make CPU profile fail
		traceFile := filepath.Join(tmpDir, "trace.out")
		cpuProfileDir := filepath.Join(tmpDir, "trace.out.prof.cpu")

		// Create a directory with the same name as the expected CPU profile file
		// This will cause the CPU profile file creation to fail
		err := os.Mkdir(cpuProfileDir, 0755)
		require.NoError(t, err)

		os.Setenv("DHCTL_TRACE", traceFile)

		shutdownFn, err := enableTrace()
		require.Error(t, err)
		require.Contains(t, err.Error(), "Failed to create pprof cpu file")
		require.NotNil(t, shutdownFn)
	})
}
