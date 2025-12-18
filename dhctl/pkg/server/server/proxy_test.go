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

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/stringsutil"
)

func TestStreamDirectorParams_Validate(t *testing.T) {
	tests := []struct {
		name        string
		params      StreamDirectorParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid params",
			params: StreamDirectorParams{
				MethodsPrefix: "/dhctl.DHCTL",
				TmpDir:        "/tmp/dhctl",
			},
			expectError: false,
		},
		{
			name: "empty methods prefix is valid",
			params: StreamDirectorParams{
				MethodsPrefix: "",
				TmpDir:        "/tmp/dhctl",
			},
			expectError: false,
		},
		{
			name: "invalid tmp dir - empty",
			params: StreamDirectorParams{
				MethodsPrefix: "/dhctl.DHCTL",
				TmpDir:        "",
			},
			expectError: true,
			errorMsg:    "tmpdir is required",
		},
		{
			name: "invalid tmp dir - root",
			params: StreamDirectorParams{
				MethodsPrefix: "/dhctl.DHCTL",
				TmpDir:        "/",
			},
			expectError: true,
			errorMsg:    "tmpdir should not be /",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewStreamDirector(t *testing.T) {
	tests := []struct {
		name        string
		params      StreamDirectorParams
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid params",
			params: StreamDirectorParams{
				MethodsPrefix: "/dhctl.DHCTL",
				TmpDir:        "/tmp/dhctl",
			},
			expectError: false,
		},
		{
			name: "invalid params",
			params: StreamDirectorParams{
				MethodsPrefix: "/dhctl.DHCTL",
				TmpDir:        "",
			},
			expectError: true,
			errorMsg:    "tmpdir is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			director, err := NewStreamDirector(tt.params)

			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
				require.Nil(t, director)
			} else {
				require.NoError(t, err)
				require.NotNil(t, director)
				require.Equal(t, tt.params, director.params)
				require.NotNil(t, director.wg)
				require.NotNil(t, director.syncWriters)
				require.NotNil(t, director.syncWriters.stdoutWriter)
				require.NotNil(t, director.syncWriters.stderrWriter)
			}
		})
	}
}

func TestStreamDirector_SocketPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dhctl-proxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	params := StreamDirectorParams{
		MethodsPrefix: "/dhctl.DHCTL",
		TmpDir:        tmpDir,
	}

	director, err := NewStreamDirector(params)
	require.NoError(t, err)

	tests := []struct {
		name         string
		directorUUID string
		expected     string
	}{
		{
			name:         "simple UUID",
			directorUUID: "12345678-1234-1234-1234-123456789abc",
			expected:     filepath.Join(tmpDir, "12345678-1234-1234-1234-123456789abc.sock"),
		},
		{
			name:         "short UUID",
			directorUUID: "abc123",
			expected:     filepath.Join(tmpDir, "abc123.sock"),
		},
		{
			name:         "empty UUID",
			directorUUID: "",
			expected:     filepath.Join(tmpDir, ".sock"),
		},
		{
			name:         "UUID with special characters",
			directorUUID: "test-uuid_123",
			expected:     filepath.Join(tmpDir, "test-uuid_123.sock"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := director.socketPath(tt.directorUUID)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestStreamDirector_TmpDirPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dhctl-proxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	params := StreamDirectorParams{
		MethodsPrefix: "/dhctl.DHCTL",
		TmpDir:        tmpDir,
	}

	director, err := NewStreamDirector(params)
	require.NoError(t, err)

	tests := []struct {
		name         string
		directorUUID string
	}{
		{
			name:         "simple UUID",
			directorUUID: "12345678-1234-1234-1234-123456789abc",
		},
		{
			name:         "short UUID",
			directorUUID: "abc123",
		},
		{
			name:         "empty UUID",
			directorUUID: "",
		},
		{
			name:         "UUID with special characters",
			directorUUID: "test-uuid_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := director.tmpDirPath(tt.directorUUID)

			// Verify the result is within the expected tmp directory
			require.True(t, strings.HasPrefix(result, tmpDir))

			// Verify the hash is computed correctly
			expectedHash := stringsutil.Sha256Encode(tt.directorUUID)
			expectedFirst10 := fmt.Sprintf("%.10s", expectedHash)
			expectedPath := filepath.Join(tmpDir, expectedFirst10)

			require.Equal(t, expectedPath, result)

			// Verify the directory name is exactly 10 characters (or less if hash is shorter)
			dirName := filepath.Base(result)
			if len(expectedHash) >= 10 {
				require.Len(t, dirName, 10)
			} else {
				require.Len(t, dirName, len(expectedHash))
			}
		})
	}
}

func TestStreamDirector_TmpDirPathConsistency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dhctl-proxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	params := StreamDirectorParams{
		MethodsPrefix: "/dhctl.DHCTL",
		TmpDir:        tmpDir,
	}

	director, err := NewStreamDirector(params)
	require.NoError(t, err)

	// Test that the same UUID always produces the same path
	uuid := "12345678-1234-1234-1234-123456789abc"

	path1 := director.tmpDirPath(uuid)
	path2 := director.tmpDirPath(uuid)
	path3 := director.tmpDirPath(uuid)

	require.Equal(t, path1, path2)
	require.Equal(t, path2, path3)
}

func TestStreamDirector_TmpDirPathUniqueness(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dhctl-proxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	params := StreamDirectorParams{
		MethodsPrefix: "/dhctl.DHCTL",
		TmpDir:        tmpDir,
	}

	director, err := NewStreamDirector(params)
	require.NoError(t, err)

	// Test that different UUIDs produce different paths
	uuid1 := "12345678-1234-1234-1234-123456789abc"
	uuid2 := "87654321-4321-4321-4321-cba987654321"
	uuid3 := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

	path1 := director.tmpDirPath(uuid1)
	path2 := director.tmpDirPath(uuid2)
	path3 := director.tmpDirPath(uuid3)

	require.NotEqual(t, path1, path2)
	require.NotEqual(t, path2, path3)
	require.NotEqual(t, path1, path3)

	// All paths should be in the same parent directory
	require.Equal(t, tmpDir, filepath.Dir(path1))
	require.Equal(t, tmpDir, filepath.Dir(path2))
	require.Equal(t, tmpDir, filepath.Dir(path3))
}

func TestStreamDirector_Wait(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dhctl-proxy-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	params := StreamDirectorParams{
		MethodsPrefix: "/dhctl.DHCTL",
		TmpDir:        tmpDir,
	}

	director, err := NewStreamDirector(params)
	require.NoError(t, err)

	// Test that Wait() doesn't block when no goroutines are running
	done := make(chan bool, 1)
	go func() {
		director.Wait()
		done <- true
	}()

	select {
	case <-done:
		// Success - Wait() returned immediately
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() should not block when no goroutines are running")
	}
}

func TestSyncWriter(t *testing.T) {
	// Create a buffer to write to
	var buf strings.Builder

	sw := &syncWriter{
		writer: &buf,
	}

	t.Run("single write", func(t *testing.T) {
		buf.Reset()

		data := []byte("test data")
		n, err := sw.Write(data)

		require.NoError(t, err)
		require.Equal(t, len(data), n)
		require.Equal(t, "test data", buf.String())
	})

	t.Run("multiple writes", func(t *testing.T) {
		buf.Reset()

		data1 := []byte("first ")
		data2 := []byte("second")

		n1, err1 := sw.Write(data1)
		n2, err2 := sw.Write(data2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.Equal(t, len(data1), n1)
		require.Equal(t, len(data2), n2)
		require.Equal(t, "first second", buf.String())
	})

	t.Run("copy from reader", func(t *testing.T) {
		buf.Reset()

		reader := strings.NewReader("data from reader")
		err := sw.copyFrom(reader)

		require.NoError(t, err)
		require.Equal(t, "data from reader", buf.String())
	})
}
