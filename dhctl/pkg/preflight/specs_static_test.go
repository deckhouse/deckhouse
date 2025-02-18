// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package preflight

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/specs_test_cpuinfo_6_cores_1_socket.txt
var cpuinfo6cores1socket []byte

//go:embed testdata/specs_test_cpuinfo_1_core_4_sockets.txt
var cpuinfo1core4sockets []byte

func TestCPUCoresCountDetection(t *testing.T) {
	tests := []struct {
		name    string
		cpuinfo []byte
		want    int
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "1 socket, 6 cores, 2 threads each",
			cpuinfo: cpuinfo6cores1socket,
			want:    12,
			wantErr: assert.NoError,
		},
		{
			name:    "4 sockets, 1 core, 1 thread each",
			cpuinfo: cpuinfo1core4sockets,
			want:    4,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := logicalCoresCountFromCPUInfo(tt.cpuinfo)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseDiskSizeInfo tests the parseDiskSizeInfo function
func TestParseDiskSizeInfo(t *testing.T) {
	tests := []struct {
		name        string
		diskInfo    string
		expected    map[string]int64
		expectedErr assert.ErrorAssertionFunc
	}{
		{
			name: "Basic case",
			diskInfo: `
123M /a
1234M /
12345M /a/b
123456M /b/c
`,
			expected: map[string]int64{
				"/a":   123,
				"/":    1234,
				"/b/c": 123456,
				"/a/b": 12345,
			},
			expectedErr: assert.NoError,
		},
		{
			name:        "Empty input",
			diskInfo:    "",
			expected:    map[string]int64{},
			expectedErr: assert.NoError,
		},
		{
			name:        "Invalid format",
			diskInfo:    "Invalid line format",
			expected:    map[string]int64{},
			expectedErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := parseDiskSizeInfo([]byte(tt.diskInfo))
			tt.expectedErr(t, err)
			if err == nil {
				for path, size := range tt.expected {
					assert.Equal(t, size, actual[path])
				}
				for path, size := range actual {
					assert.Equal(t, tt.expected[path], size)
				}
			}
		})
	}
}

// TestGetRelationFolderByDisk tests the getRelationFolderByDisk function
func TestGetRelationFolderByDisk(t *testing.T) {
	tests := []struct {
		name        string
		disks       []string
		folders     []string
		expected    map[string][]string
		expectedErr assert.ErrorAssertionFunc
	}{
		{
			name:    "Basic case",
			disks:   []string{"/", "/a", "/b", "/a/b", "/b/a"},
			folders: []string{"/", "/a", "/b", "/a/b", "/b/a", "/d", "/d/d", "/d/a", "/b/b/b", "/b/a/b", "/a/a/a", "/a/b/a"},
			expected: map[string][]string{
				"/":    {"/", "/d", "/d/d", "/d/a"},
				"/a":   {"/a", "/a/a/a"},
				"/b":   {"/b", "/b/b/b"},
				"/a/b": {"/a/b", "/a/b/a"},
				"/b/a": {"/b/a", "/b/a/b"},
			},
			expectedErr: assert.NoError,
		},
		{
			name:        "Empty input",
			disks:       []string{},
			folders:     []string{},
			expected:    nil,
			expectedErr: assert.NoError,
		},
		{
			name:        "Unknown folder",
			disks:       []string{},
			folders:     []string{"/unknown/temp"},
			expected:    nil,
			expectedErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getRelationFoldersByDisk(tt.disks, tt.folders)
			tt.expectedErr(t, err)

			if err == nil {
				for disk, folderList := range tt.expected {
					assert.ElementsMatch(t, folderList, actual[disk])
				}
				for disk, folderList := range actual {
					assert.ElementsMatch(t, tt.expected[disk], folderList)
				}
			}
		})
	}
}
