//go:build linux

/*
Copyright 2026 Flant JSC

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

package verity

import (
	"testing"

	"github.com/containerd/go-dmverity/pkg/dm"
	"github.com/stretchr/testify/assert"
)

func TestIsIncompleteMapper(t *testing.T) {
	const uuid = "CRYPT-VERITY-0123456789abcdef0123456789abcdef-console"

	tests := []struct {
		name   string
		status dm.DeviceStatus
		want   bool
	}{
		{name: "inactive table only", status: dm.DeviceStatus{InactivePresent: true, UUID: uuid}, want: true},
		{name: "no table", status: dm.DeviceStatus{UUID: uuid}, want: true},
		{name: "live table", status: dm.DeviceStatus{ActivePresent: true, UUID: uuid}, want: false},
		{name: "in use", status: dm.DeviceStatus{OpenCount: 1, UUID: uuid}, want: false},
		{name: "not created by veritysetup", status: dm.DeviceStatus{UUID: "LVM-0123456789abcdef"}, want: false},
		{name: "no uuid", status: dm.DeviceStatus{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isIncompleteMapper(tt.status))
		})
	}
}
