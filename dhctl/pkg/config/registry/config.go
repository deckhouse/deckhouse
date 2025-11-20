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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_docker "github.com/deckhouse/deckhouse/go_lib/registry/docker"
)

const (
	SchemeHTTP      SchemeType = "HTTP"
	SchemeHTTPS     SchemeType = "HTTPS"
	CRIContainerdV1 CRIType    = "Containerd"
	CRIContainerdV2 CRIType    = "ContainerdV2"

	defaultImagesRepo = "registry.deckhouse.io/deckhouse/ce"
	defaultScheme     = SchemeHTTPS
	licenseUsername   = "license-token"
)

var (
	SupportedCRI = []CRIType{CRIContainerdV1, CRIContainerdV2}
)

type Config struct {
	Mode       registry_const.ModeType      `json:"mode" yaml:"mode"`
	ImagesRepo string                       `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType                   `json:"scheme" yaml:"scheme"`
	CA         string                       `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string                       `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string                       `json:"password,omitempty" yaml:"password,omitempty"`
	CheckMode  registry_const.CheckModeType `json:"checkMode,omitempty" yaml:"checkMode,omitempty"`

	DefaultCRI CRIType `json:"defaultCRI" yaml:"defaultCRI"`
}

func NewConfig(moduleConfig *DeckhouseSettings, initConfig *InitConfig, defaultCRI string) (Config, error) {
	// if moduleConfig != nil && initConfig != nil {
	// 	return Config{}, fmt.Errorf("conflicting registry settings: specify in only one of 'initConfig' or 'moduleConfig/deckhouse'")
	// }

	config := Config{}
	switch {
	case moduleConfig != nil:
		if err := config.fromDeckhouseSettings(*moduleConfig); err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from moduleConfig deckhouse: %w", err)
		}
	case initConfig != nil:
		if err := config.unmanagedFromInitConfig(*initConfig); err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from initConfig: %w", err)
		}
	default:
		config.Mode = registry_const.ModeUnmanaged
		config.ImagesRepo = defaultImagesRepo
		config.Scheme = defaultScheme
	}

	config.DefaultCRI = defaultCRI
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("failed to validate registry settings: %w", err)
	}
	return config, nil
}

func (cfg *Config) ConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{cfg: cfg}
}

func (cfg *Config) unmanagedFromInitConfig(initConfig InitConfig) error {
	initConfig.ImagesRepo = strings.TrimRight(initConfig.ImagesRepo, "/")

	// Validate and pars dockerCfg
	address, _ := addressAndPathFromImagesRepo(initConfig.ImagesRepo)
	if err := validateRegistryDockerCfg(initConfig.RegistryDockerCfg, address); err != nil {
		return fmt.Errorf("failed to validate registryDockerCfg: %w", err)
	}
	dockerCfgDecode, err := base64.StdEncoding.DecodeString(initConfig.RegistryDockerCfg)
	if err != nil {
		return fmt.Errorf("failed to decode registryDockerCfg: %w", err)
	}
	username, password, err := registry_docker.CredsFromDockerCfg(dockerCfgDecode, address)
	if err != nil {
		return err
	}

	cfg.Mode = registry_const.ModeUnmanaged
	cfg.ImagesRepo = initConfig.ImagesRepo
	cfg.Scheme = schemeFromString(initConfig.RegistryScheme)
	cfg.CA = initConfig.RegistryCA
	cfg.Username = username
	cfg.Password = password
	return nil
}

func (cfg *Config) fromDeckhouseSettings(moduleConfig DeckhouseSettings) error {
	cfg.Mode = moduleConfig.Mode
	switch moduleConfig.Mode {
	case registry_const.ModeDirect:
		if moduleConfig.Direct == nil {
			return fmt.Errorf("field 'direct' is required when mode is 'Direct'")
		}
		direct := moduleConfig.Direct
		cfg.ImagesRepo = strings.TrimRight(direct.ImagesRepo, "/")
		cfg.Scheme = schemeFromString(direct.Scheme)
		cfg.CA = direct.CA
		cfg.Username = direct.Username
		cfg.Password = direct.Password
		if direct.License != "" {
			cfg.Username = licenseUsername
			cfg.Password = direct.License
		}
	case registry_const.ModeUnmanaged:
		if moduleConfig.Unmanaged == nil {
			return fmt.Errorf("field 'Unmanaged' is required when mode is 'Unmanaged'")
		}
		unmanaged := moduleConfig.Unmanaged
		cfg.ImagesRepo = strings.TrimRight(unmanaged.ImagesRepo, "/")
		cfg.Scheme = schemeFromString(unmanaged.Scheme)
		cfg.CA = unmanaged.CA
		cfg.Username = unmanaged.Username
		cfg.Password = unmanaged.Password
		if unmanaged.License != "" {
			cfg.Username = licenseUsername
			cfg.Password = unmanaged.License
		}
	}
	return nil
}

func (cfg *Config) toDeckhouseSettings() (DeckhouseSettings, error) {
	switch cfg.Mode {
	case registry_const.ModeDirect:
		direct := DirectModeSettings{
			CheckMode:  cfg.CheckMode,
			ImagesRepo: cfg.ImagesRepo,
			Scheme:     cfg.Scheme,
			CA:         cfg.CA,
		}
		if cfg.Username == licenseUsername {
			direct.License = cfg.Password
		} else {
			direct.Username = cfg.Username
			direct.Password = cfg.Password
		}
		return DeckhouseSettings{
			Mode:   registry_const.ModeDirect,
			Direct: &direct,
		}, nil
	case registry_const.ModeUnmanaged:
		unmanaged := UnmanagedModeSettings{
			CheckMode:  cfg.CheckMode,
			ImagesRepo: cfg.ImagesRepo,
			Scheme:     cfg.Scheme,
			CA:         cfg.CA,
		}
		if cfg.Username == licenseUsername {
			unmanaged.License = cfg.Password
		} else {
			unmanaged.Username = cfg.Username
			unmanaged.Password = cfg.Password
		}
		return DeckhouseSettings{
			Mode:      registry_const.ModeUnmanaged,
			Unmanaged: &unmanaged,
		}, nil
	}
	return DeckhouseSettings{}, ErrUnknownMode
}

func (cfg Config) isModuleEnabled() bool {
	return slices.Contains(SupportedCRI, cfg.DefaultCRI)
}

func (cfg Config) Validate() error {
	// Check registry spec
	if err := validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Mode,
			validation.Required.Error("field 'Mode' is required"),
			validation.In(registry_const.ModeDirect, registry_const.ModeUnmanaged).
				Error(fmt.Sprintf("unknown registry mode: %s", cfg.Mode)),
		),
		validation.Field(&cfg.Mode,
			validation.When(cfg.CheckMode != "",
				validation.In(registry_const.CheckModeDefault, registry_const.CheckModeRelax).
					Error(fmt.Sprintf("unknown registry check mode: %s", cfg.Mode))),
		),
		validation.Field(&cfg.ImagesRepo,
			validation.Required.Error("field 'ImagesRepo' is required"),
		),
		validation.Field(&cfg.Scheme,
			validation.In(SchemeHTTP, SchemeHTTPS).
				Error(fmt.Sprintf("invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", cfg.Scheme)),
		),
		validation.Field(&cfg.Username,
			validation.When(cfg.Password != "",
				validation.Required.Error("username is required when password is provided"),
			),
		),
		validation.Field(&cfg.Password,
			validation.When(cfg.Username != "",
				validation.Required.Error("password is required when username is provided"),
			),
		),
		validation.Field(&cfg.CA,
			validation.When(cfg.Scheme == SchemeHTTP,
				validation.Empty.Error("CA is not allowed when scheme is 'HTTP'"),
			),
		),
	); err != nil {
		return err
	}

	// Check defaultCRI
	switch cfg.Mode {
	case registry_const.ModeDirect:
		if err := validation.ValidateStruct(&cfg,
			validation.Field(&cfg.DefaultCRI,
				validation.In(toSliceOfInterface(SupportedCRI)...).
					Error(fmt.Sprintf(
						"unable to use defaultCRI '%s'; only '%v' are supported for '%s' mode",
						cfg.DefaultCRI, SupportedCRI, cfg.Mode)),
			),
		); err != nil {
			return err
		}
	}
	return nil
}

func toSliceOfInterface[T any](slice []T) []interface{} {
	result := make([]interface{}, 0, len(slice))
	for _, v := range slice {
		result = append(result, v)
	}
	return result
}

func validateRegistryDockerCfg(cfg string, repo string) error {
	if cfg == "" {
		return fmt.Errorf("can't be empty")
	}

	regcrd, err := base64.StdEncoding.DecodeString(cfg)
	if err != nil {
		return fmt.Errorf("unable to decode registryDockerCfg: %w", err)
	}

	var creds struct {
		Auths map[string]interface{} `json:"auths"`
	}

	if err = json.Unmarshal(regcrd, &creds); err != nil {
		return fmt.Errorf("unable to unmarshal docker credentials: %w", err)
	}

	// The regexp match string with this pattern:
	// ^([a-z]|\d)+ - string starts with a [a-z] letter or a number
	// (\.?|\-?) - next symbol might be '.' or '-' and repeated zero or one times
	// (([a-z]|\d)+(\.|\-|))* - middle part of string might have [a-z] letters, numbers, '.' or ':',
	// and moreover '.' or ':' symbols can't be doubled or goes next to each other
	// ([a-z]|\d+|([a-z]|\d)\:\d+)$ - string might be ended by [a-z] letter or number (if we have single host) or
	// [a-z] letter or number with ':' symbol, and moreover there might be only numbers after ':' symbol
	regx, err := regexp.Compile(`^([a-z]|\d)+(\.?|\-?)(([a-z]|\d)+(\.|\-|))*([a-z]|\d+|([a-z]|\d)\:\d+)$`)
	if err != nil {
		return fmt.Errorf("unable to compile regexp by pattern: %w", err)
	}

	for k := range creds.Auths {
		if !regx.MatchString(k) {
			return fmt.Errorf("invalid registryDockerCfg. Your auths host \"%s\" should be similar to \"your.private.registry.example.com\"", k)
		}
	}

	for k := range creds.Auths {
		if k == repo {
			return nil
		}
	}
	return fmt.Errorf("incorrect registryDockerCfg. It must contain auths host {\"auths\": { \"%s\": {}}}", repo)
}
