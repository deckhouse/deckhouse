/*
Copyright 2025 Flant JSC

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

package initconfig

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	"github.com/deckhouse/deckhouse/go_lib/registry/helpers"
	module_config "github.com/deckhouse/deckhouse/go_lib/registry/models/module-config"
)

type Config struct {
	ImagesRepo        string `json:"imagesRepo" yaml:"imagesRepo"`
	RegistryScheme    string `json:"registryScheme" yaml:"registryScheme"`
	RegistryDockerCfg string `json:"registryDockerCfg,omitempty" yaml:"registryDockerCfg,omitempty"`
	RegistryCA        string `json:"registryCA,omitempty" yaml:"registryCA,omitempty"`
}

func (config *Config) Correct() {
	config.ImagesRepo = strings.TrimRight(strings.TrimSpace(config.ImagesRepo), "/")
	if strings.TrimSpace(config.ImagesRepo) == "" {
		config.ImagesRepo = constant.CEImagesRepo
	}
	if strings.TrimSpace(config.RegistryScheme) == "" {
		config.RegistryScheme = constant.CEScheme
	}
}

func (config Config) ToRegistrySettings() (module_config.RegistrySettings, error) {
	config.Correct()

	registrySettings := module_config.RegistrySettings{
		ImagesRepo: config.ImagesRepo,
		Scheme:     constant.ToScheme(config.RegistryScheme),
		CA:         config.RegistryCA,
	}

	if config.RegistryDockerCfg == "" {
		return registrySettings, nil
	}

	// Validate and pars dockerCfg
	address, _ := helpers.SplitAddressAndPath(config.ImagesRepo)
	if err := validateRegistryDockerCfg(config.RegistryDockerCfg, address); err != nil {
		return module_config.RegistrySettings{}, fmt.Errorf("failed to validate registryDockerCfg: %w", err)
	}
	dockerCfgDecode, err := base64.StdEncoding.DecodeString(config.RegistryDockerCfg)
	if err != nil {
		return module_config.RegistrySettings{}, fmt.Errorf("failed to decode registryDockerCfg: %w", err)
	}
	username, password, err := helpers.CredsFromDockerCfg(dockerCfgDecode, address)
	if err != nil {
		return module_config.RegistrySettings{}, err
	}
	registrySettings.Username = username
	registrySettings.Password = password
	return registrySettings, nil
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
