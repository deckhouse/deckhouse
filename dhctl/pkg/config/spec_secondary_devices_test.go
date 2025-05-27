// Copyright 2024 Flant JSC
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

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewProviderSecondaryDevicesConfigFromData(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "Valid config",
			config: `
RegistryDataDeviceEnable: true
`,
			wantErr: assert.NoError,
		},
		{
			name:    "Empty config",
			config:  ``,
			wantErr: assert.NoError,
		},
		{
			name: "Empty field",
			config: `


`,
			wantErr: assert.NoError,
		},
		{
			name: "Invalid config",
			config: `
Enable: true
`,
			wantErr: assert.Error,
		},
		{
			name: "Invalid yaml",
			config: `
RegistryDataDeviceEnable\n true
`,
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProviderSecondaryDevicesConfigFromData([]byte(tt.config))
			tt.wantErr(t, err)
		})
	}
}

func TestProviderSecondaryDevicesConfigValidateRegistryDataDevice(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		config   ProviderSecondaryDevicesConfig
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name: "Providers supporting secondary data devices with enable data device option",
			config: ProviderSecondaryDevicesConfig{
				RegistryDataDeviceEnable: true,
			},
			provider: "YaNdEx",
			wantErr:  assert.NoError,
		},
		{
			name: "Providers supporting secondary data devices with disable data device option",
			config: ProviderSecondaryDevicesConfig{
				RegistryDataDeviceEnable: false,
			},
			provider: "Yandex",
			wantErr:  assert.NoError,
		},
		{
			name: "Providers unsupporting secondary data devices with enable data device option",
			config: ProviderSecondaryDevicesConfig{
				RegistryDataDeviceEnable: true,
			},
			provider: "NonYandex",
			wantErr:  assert.Error,
		},
		{
			name: "Providers unsupporting secondary data devices with disable data device option",
			config: ProviderSecondaryDevicesConfig{
				RegistryDataDeviceEnable: false,
			},
			provider: "NonYandex",
			wantErr:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, tt.config.ValidateRegistryDataDevice(tt.provider))
		})
	}
}
