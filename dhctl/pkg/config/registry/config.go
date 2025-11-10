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
	SchemeHTTP  SchemeType = "HTTP"
	SchemeHTTPS SchemeType = "HTTPS"

	CRIContainerdV1 CRIType = "Containerd"
	CRIContainerdV2 CRIType = "ContainerdV2"

	defaultImagesRepo = "registry.deckhouse.io/deckhouse/ce"
	defaultScheme     = SchemeHTTPS

	licenseUsername = "license-token"
)

type Config struct {
	ModuleConfig ModuleConfig
	DefaultCRI   CRIType
}

type InitConfig struct {
	ImagesRepo        string `json:"imagesRepo" yaml:"imagesRepo"`
	RegistryDockerCfg string `json:"registryDockerCfg,omitempty" yaml:"registryDockerCfg,omitempty"`
	RegistryCA        string `json:"registryCA,omitempty" yaml:"registryCA,omitempty"`
	RegistryScheme    string `json:"registryScheme,omitempty" yaml:"registryScheme,omitempty"`
}

type ModuleConfig struct {
	Mode      registry_const.ModeType `json:"mode" yaml:"mode"`
	Direct    *DirectModeConfig       `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeConfig    `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
}

type DirectModeConfig struct {
	ImagesRepo string     `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType `json:"scheme" yaml:"scheme"`
	CA         string     `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string     `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string     `json:"password,omitempty" yaml:"password,omitempty"`
	License    string     `json:"license,omitempty" yaml:"license,omitempty"`
}

type UnmanagedModeConfig struct {
	ImagesRepo string     `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType `json:"scheme" yaml:"scheme"`
	CA         string     `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string     `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string     `json:"password,omitempty" yaml:"password,omitempty"`
	License    string     `json:"license,omitempty" yaml:"license,omitempty"`
}

type SchemeType = string

type CRIType = string

func NewConfig(
	moduleConfig *ModuleConfig,
	initConfig *InitConfig,
	defaultCRI string,
) (Config, error) {
	var finalModuleConfig ModuleConfig

	switch {
	case moduleConfig != nil:
		if err := finalModuleConfig.fromDeckhouseModuleConfig(*moduleConfig); err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from moduleConfig deckhouse: %w", err)
		}
	case initConfig != nil:
		if err := finalModuleConfig.unmanagedFromInitConfig(*initConfig); err != nil {
			return Config{}, fmt.Errorf("failed to get registry settings from initConfig: %w", err)
		}
	default:
		finalModuleConfig = ModuleConfig{
			Mode: registry_const.ModeUnmanaged,
			Unmanaged: &UnmanagedModeConfig{
				ImagesRepo: defaultImagesRepo,
				Scheme:     defaultScheme,
				CA:         "",
				Username:   "",
				Password:   "",
				License:    "",
			},
		}
	}

	config := Config{
		ModuleConfig: finalModuleConfig,
		DefaultCRI:   defaultCRI,
	}
	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("Invalid registry config: %w", err)
	}
	return config, nil
}

func (cfg *ModuleConfig) fromDeckhouseModuleConfig(moduleConfig ModuleConfig) error {
	switch {
	case moduleConfig.Direct != nil:
		moduleConfig.Direct.ImagesRepo = strings.TrimRight(moduleConfig.Direct.ImagesRepo, "/")
	case moduleConfig.Unmanaged != nil:
		moduleConfig.Unmanaged.ImagesRepo = strings.TrimRight(moduleConfig.Unmanaged.ImagesRepo, "/")
	}
	*cfg = moduleConfig
	return nil
}

func (cfg *ModuleConfig) unmanagedFromInitConfig(initConfig InitConfig) error {
	initConfig.ImagesRepo = strings.TrimRight(initConfig.ImagesRepo, "/")
	address, _ := addressAndPathFromImagesRepo(initConfig.ImagesRepo)

	err := validateRegistryDockerCfg(initConfig.RegistryDockerCfg, address)
	if err != nil {
		return err
	}

	dockerCfgDecode, err := base64.StdEncoding.DecodeString(initConfig.RegistryDockerCfg)
	if err != nil {
		return fmt.Errorf("unable to decode registryDockerCfg: %w", err)
	}

	username, password, err := registry_docker.CredsFromDockerCfg(dockerCfgDecode, address)
	if err != nil {
		return err
	}

	*cfg = ModuleConfig{
		Mode: registry_const.ModeUnmanaged,
		Unmanaged: &UnmanagedModeConfig{
			ImagesRepo: initConfig.ImagesRepo,
			Scheme:     SchemeFromString(initConfig.RegistryScheme),
			CA:         initConfig.RegistryCA,
			Username:   username,
			Password:   password,
		},
	}
	return nil
}

func (cfg *Config) ConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{cfg: cfg}
}

func (cfg Config) isModuleEnable() bool {
	// If cri int allowed list -> use module registry
	return slices.Contains(
		[]CRIType{CRIContainerdV1, CRIContainerdV2},
		CRIType(cfg.DefaultCRI))
}

func (cfg Config) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.ModuleConfig),
		validation.Field(&cfg.DefaultCRI,
			validation.When(
				slices.Contains([]registry_const.ModeType{registry_const.ModeDirect}, cfg.ModuleConfig.Mode),
				validation.In(CRIContainerdV1, CRIContainerdV2).
					Error(fmt.Sprintf("unable to use defaultCRI '%s'; only 'Containerd' and 'ContainerdV2' are supported for 'Direct' mode", cfg.DefaultCRI)),
			),
		),
	)
}

func (cfg ModuleConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.Mode,
			validation.In(registry_const.ModeDirect, registry_const.ModeUnmanaged).
				Error(fmt.Sprintf("Unknown registry mode: %s", cfg.Mode)),
		),
		validation.Field(&cfg.Direct,
			validation.When(cfg.Mode == registry_const.ModeDirect,
				validation.NotNil,
				validation.Required.Error("Field 'direct' is required when mode is 'Direct'"),
			).Else(
				validation.Nil.Error("Field 'direct' must be empty when mode is not 'Direct'"),
			),
		),
		validation.Field(&cfg.Unmanaged,
			validation.When(cfg.Mode == registry_const.ModeUnmanaged,
				validation.NotNil,
				validation.Required.Error("Field 'unmanaged' is required when mode is 'Unmanaged'"),
			).Else(
				validation.Nil.Error("Field 'unmanaged' must be empty when mode is not 'Unmanaged'"),
			),
		),
	)
}

func (cfg DirectModeConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.ImagesRepo,
			validation.Required.Error("Field 'imagesRepo' is required"),
		),
		validation.Field(&cfg.Scheme,
			validation.In(SchemeHTTP, SchemeHTTPS).
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", cfg.Scheme)),
		),
		validation.Field(&cfg.Username,
			validation.When(cfg.Password != "",
				validation.Required.Error("Username is required when password is provided"),
			),
		),
		validation.Field(&cfg.Password,
			validation.When(cfg.Username != "",
				validation.Required.Error("Password is required when username is provided"),
			),
		),
		validation.Field(&cfg.License,
			validation.When(cfg.Username != "" || cfg.Password != "",
				validation.Empty.Error("License field must be empty when using credentials (username/password)"),
			),
		),
		validation.Field(&cfg.CA,
			validation.When(cfg.Scheme == SchemeHTTP,
				validation.Empty.Error("CA is not allowed when scheme is 'HTTP'"),
			),
		),
	)
}

func (cfg UnmanagedModeConfig) Validate() error {
	return validation.ValidateStruct(&cfg,
		validation.Field(&cfg.ImagesRepo,
			validation.Required.Error("Field 'imagesRepo' is required"),
		),
		validation.Field(&cfg.Scheme,
			validation.In(SchemeHTTP, SchemeHTTPS).
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", cfg.Scheme)),
		),
		validation.Field(&cfg.Username,
			validation.When(cfg.Password != "",
				validation.Required.Error("Username is required when password is provided"),
			),
		),
		validation.Field(&cfg.Password,
			validation.When(cfg.Username != "",
				validation.Required.Error("Password is required when username is provided"),
			),
		),
		validation.Field(&cfg.License,
			validation.When(cfg.Username != "" || cfg.Password != "",
				validation.Empty.Error("License field must be empty when using credentials (username/password)"),
			),
		),
		validation.Field(&cfg.CA,
			validation.When(cfg.Scheme == SchemeHTTP,
				validation.Empty.Error("CA certificate is not allowed when scheme is 'HTTP'"),
			),
		),
	)
}

func (cfg *DirectModeConfig) UsernamePassword() (username string, password string) {
	if cfg.License != "" {
		return licenseUsername, cfg.License
	}
	return cfg.Username, cfg.Password
}

func (cfg *UnmanagedModeConfig) UsernamePassword() (username string, password string) {
	if cfg.License != "" {
		return licenseUsername, cfg.License
	}
	return cfg.Username, cfg.Password
}

func SchemeFromString(scheme string) SchemeType {
	if strings.EqualFold(scheme, SchemeHTTP) {
		return SchemeHTTP
	}
	return SchemeHTTPS
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
