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
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type TestConfigUpdateRegistrySettings func(*module_config.RegistrySettings)
type TestConfigUpdateModuleEnabled func() bool
type TestConfigUpdateMode func() constant.ModeType

func NewTestConfig(opts ...interface{}) Config {
	registrySettings := module_config.RegistrySettings{
		ImagesRepo: constant.CEImagesRepo,
		Scheme:     constant.CEScheme,
	}

	mode := constant.ModeUnmanaged
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

	var dekhouseSettings module_config.DeckhouseSettings
	switch mode {
	case constant.ModeDirect:
		dekhouseSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &registrySettings,
		}
		moduleEnabled = true
	default:
		dekhouseSettings = module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		}
	}

	dekhouseSettings, err := newDeckhouseSettings(&dekhouseSettings, nil)
	if err != nil {
		panic(err)
	}

	settings, err := newModeSettings(dekhouseSettings)
	if err != nil {
		panic(err)
	}
	return Config{
		Settings:          settings,
		DeckhouseSettings: dekhouseSettings,
		ModuleEnabled:     moduleEnabled,
	}
}

func WithImagesRepo(repo string) TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.ImagesRepo = repo
	}
}

func WithSchemeHTTP() TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTP
	}
}

func WithSchemeHTTPS() TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTPS
	}
}

func WithCredentials(username, password string) TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Username = username
		rs.Password = password
	}
}

func WithCA(ca string) TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.CA = ca
	}
}

func WithLicense(license string) TestConfigUpdateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
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
	return func() constant.ModeType {
		return constant.ModeDirect
	}
}

func WithModeUnmanaged() TestConfigUpdateMode {
	return func() constant.ModeType {
		return constant.ModeUnmanaged
	}
}
