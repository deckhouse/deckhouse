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

package registrymocks

import (
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
)

type (
	updateRegistrySettings func(*module_config.RegistrySettings)
	updateLegacyMode       func() bool
	updateMode             func() constant.ModeType
)

func ConfigBuilder(opts ...any) registry.Config {
	var (
		mode             = constant.ModeUnmanaged
		legacyMode       = false
		registrySettings = module_config.RegistrySettings{
			ImagesRepo: constant.CEImagesRepo,
			Scheme:     constant.CEScheme,
		}
	)

	for _, opt := range opts {
		switch fn := opt.(type) {
		case updateRegistrySettings:
			fn(&registrySettings)

		case updateLegacyMode:
			legacyMode = fn()

		case updateMode:
			mode = fn()
		}
	}

	var deckhouseSettings module_config.DeckhouseSettings

	switch mode {
	case constant.ModeDirect:
		deckhouseSettings = module_config.DeckhouseSettings{
			Mode:   constant.ModeDirect,
			Direct: &registrySettings,
		}

	default:
		deckhouseSettings = module_config.DeckhouseSettings{
			Mode:      constant.ModeUnmanaged,
			Unmanaged: &registrySettings,
		}
	}

	var config registry.Config

	if err := config.Process(deckhouseSettings, legacyMode); err != nil {
		panic(err)
	}

	return config
}

func WithImagesRepo(repo string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.ImagesRepo = repo
	}
}

func WithSchemeHTTP() updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTP
	}
}

func WithSchemeHTTPS() updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Scheme = constant.SchemeHTTPS
	}
}

func WithCredentials(username, password string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.Username = username
		rs.Password = password
	}
}

func WithCA(ca string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.CA = ca
	}
}

func WithLicense(license string) updateRegistrySettings {
	return func(rs *module_config.RegistrySettings) {
		rs.License = license
	}
}

func WithModeDirect() updateMode {
	return func() constant.ModeType {
		return constant.ModeDirect
	}
}

func WithModeUnmanaged() updateMode {
	return func() constant.ModeType {
		return constant.ModeUnmanaged
	}
}

func WithLegacyMode() updateLegacyMode {
	return func() bool {
		return true
	}
}
