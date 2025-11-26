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

package registry

import (
	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry/types"
)

type TestConfigUpdateRegistrySettings func(*types.RegistrySettings)
type TestConfigUpdateModuleEnabled func() bool
type TestConfigUpdateMode func() registry_const.ModeType

func NewTestConfig(opts ...interface{}) Config {
	registrySettings := types.RegistrySettings{
		ImagesRepo: DefaultImagesRepo,
		Scheme:     DefaultScheme,
	}

	var mode = registry_const.ModeUnmanaged
	var settings types.DeckhouseSettings
	var modeObj Mode
	moduleEnabled := true
	for _, opt := range opts {
		switch fn := opt.(type) {
		case TestConfigUpdateRegistrySettings:
			fn(&registrySettings)
		case TestConfigUpdateModuleEnabled:
			moduleEnabled = fn()
		case TestConfigUpdateMode:
			mode = fn()
		}
	}

	switch mode {
	case registry_const.ModeDirect:
		settings = types.DeckhouseSettings{
			Mode: registry_const.ModeDirect,
			Direct: &types.DirectModeSettings{
				RegistrySettings: registrySettings,
			},
		}
		remote := types.Data{}
		remote.FromRegistrySettings(registrySettings)
		modeObj = &DirectMode{Remote: remote}
		// UpdateModuleEnabled is ignored for Direct mode (module always enabled)
		moduleEnabled = true

	default: // Unmanaged mode
		settings = types.DeckhouseSettings{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &types.UnmanagedModeSettings{
				RegistrySettings: registrySettings,
			},
		}
		remote := types.Data{}
		remote.FromRegistrySettings(registrySettings)
		modeObj = &UnmanagedMode{Remote: remote}
	}

	return Config{
		Mode:          modeObj,
		ModuleEnabled: moduleEnabled,
		Settings:      settings,
	}
}

func WithImagesRepo(repo string) TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.ImagesRepo = repo
	}
}

func WithSchemeHTTP() TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.Scheme = types.SchemeHTTP
	}
}

func WithSchemeHTTPS() TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.Scheme = types.SchemeHTTPS
	}
}

func WithCredentials(username, password string) TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.Username = username
		rs.Password = password
	}
}

func WithCA(ca string) TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.CA = ca
	}
}

func WithLicense(license string) TestConfigUpdateRegistrySettings {
	return func(rs *types.RegistrySettings) {
		rs.License = license
	}
}

func WithModuleEnable() TestConfigUpdateModuleEnabled {
	return func() bool {
		return true
	}
}

func WithModuleDisable() TestConfigUpdateModuleEnabled {
	return func() bool {
		return false
	}
}

func WithModeDirect() TestConfigUpdateMode {
	return func() registry_const.ModeType {
		return registry_const.ModeDirect
	}
}

func WithModeUnmanaged() TestConfigUpdateMode {
	return func() registry_const.ModeType {
		return registry_const.ModeUnmanaged
	}
}
