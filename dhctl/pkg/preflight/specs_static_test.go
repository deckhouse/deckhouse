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
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
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

func TestCheckStaticNodeSystemRequirementsByBundle(t *testing.T) {
	tests := []struct {
		name          string
		bundle        string
		ramKB         int
		cpuInfo       string
		expectedError string
	}{
		{
			name:    "minimal bundle accepts 2 CPU and 4GiB RAM",
			bundle:  config.MinimalBundle,
			ramKB:   4096 * 1024,
			cpuInfo: testCPUInfoWithProcessors(2),
		},
		{
			name:          "default bundle rejects 2 CPU and 4GiB RAM",
			bundle:        config.DefaultBundle,
			ramKB:         4096 * 1024,
			cpuInfo:       testCPUInfoWithProcessors(2),
			expectedError: "at least 4 CPU(s)",
		},
		{
			name:          "minimal bundle rejects one CPU",
			bundle:        config.MinimalBundle,
			ramKB:         4096 * 1024,
			cpuInfo:       testCPUInfoWithProcessors(1),
			expectedError: "at least 2 CPU(s)",
		},
		{
			name:          "minimal bundle rejects less than minimal RAM threshold",
			bundle:        config.MinimalBundle,
			ramKB:         3072 * 1024,
			cpuInfo:       testCPUInfoWithProcessors(2),
			expectedError: "at least 3584 MiB of RAM",
		},
		{
			name:          "empty bundle uses default requirements",
			bundle:        "",
			ramKB:         4096 * 1024,
			cpuInfo:       testCPUInfoWithProcessors(2),
			expectedError: "at least 4 CPU(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeMock := &mockNodeInterface{}
			memCmdMock := &mockCommand{}
			cpuCmdMock := &mockCommand{}

			nodeMock.
				On("Command", "cat", []string{"/proc/meminfo"}).
				Return(memCmdMock).
				Once()
			nodeMock.
				On("Command", "cat", []string{"/proc/cpuinfo"}).
				Return(cpuCmdMock).
				Once()

			memCmdMock.
				On("Output", mock.Anything).
				Return([]byte(fmt.Sprintf("MemTotal:       %d kB\n", tt.ramKB)), []byte{}, nil).
				Once()
			cpuCmdMock.
				On("Output", mock.Anything).
				Return([]byte(tt.cpuInfo), []byte{}, nil).
				Once()

			checker := &Checker{
				nodeInterface: nodeMock,
				installConfig: &config.DeckhouseInstaller{Bundle: tt.bundle},
			}

			err := checker.CheckStaticNodeSystemRequirements(context.Background())
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			nodeMock.AssertExpectations(t)
			memCmdMock.AssertExpectations(t)
			cpuCmdMock.AssertExpectations(t)
		})
	}
}

func testCPUInfoWithProcessors(count int) string {
	output := ""
	for i := 0; i < count; i++ {
		output += fmt.Sprintf("processor\t: %d\nvendor_id\t: GenuineIntel\n\n", i)
	}
	return output
}
