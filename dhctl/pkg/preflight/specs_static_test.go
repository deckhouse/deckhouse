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
