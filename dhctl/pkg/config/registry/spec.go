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
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	registry_docker "github.com/deckhouse/deckhouse/go_lib/registry/docker"
)

var (
	SchemeHTTP  Scheme = "HTTP"
	SchemeHTTPS Scheme = "HTTPS"
)

type Spec struct {
	Mode      registry_const.ModeType `json:"mode" yaml:"mode"`
	Direct    *DirectModeSpec         `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeSpec      `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
}

type Scheme = string

type DirectModeSpec struct {
	ImagesRepo string `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     Scheme `json:"scheme" yaml:"scheme"`
	CA         string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
}

type UnmanagedModeSpec struct {
	ImagesRepo string `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     Scheme `json:"scheme" yaml:"scheme"`
	CA         string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
}

type InitConfigSpec struct {
	ImagesRepo        string `json:"imagesRepo" yaml:"imagesRepo"`
	RegistryDockerCfg string `json:"registryDockerCfg,omitempty" yaml:"registryDockerCfg,omitempty"`
	RegistryCA        string `json:"registryCA,omitempty" yaml:"registryCA,omitempty"`
	RegistryScheme    string `json:"registryScheme,omitempty" yaml:"registryScheme,omitempty"`
}

func (s *Spec) fromInitConfig(initConfig InitConfigSpec) error {
	initConfig.ImagesRepo = strings.TrimRight(initConfig.ImagesRepo, "/")
	address, _ := addressAndPathFromImagesRepo(initConfig.ImagesRepo)

	err := validateRegistryDockerCfg(initConfig.RegistryDockerCfg, address)
	if err != nil {
		return err
	}
	username, password, err := registry_docker.CredsFromDockerCfg([]byte(initConfig.RegistryDockerCfg), address)
	if err != nil {
		return err
	}

	spec := Spec{
		Mode: registry_const.ModeUnmanaged,
		Unmanaged: &UnmanagedModeSpec{
			ImagesRepo: initConfig.ImagesRepo,
			Scheme:     SchemeFromString(initConfig.RegistryScheme),
			CA:         initConfig.RegistryCA,
			Username:   username,
			Password:   password,
		},
	}

	if err := spec.Validate(); err != nil {
		return err
	}

	*s = spec
	return nil
}

func (s *Spec) fromDeckhouseSettings(deckhouseSettings map[string]any) error {
	var err error
	var spec Spec

	rawSpec, ok := deckhouseSettings["registry"]
	if !ok {
		return fmt.Errorf("empty registry spec in deckhouse moduleConfig")
	}

	jsonSpec, err := json.Marshal(rawSpec)
	if err != nil {
		return fmt.Errorf("failed to get registry config: %w", err)
	}

	err = yaml.Unmarshal(jsonSpec, &spec)
	if err != nil {
		return fmt.Errorf("failed to get registry config: %w", err)
	}

	switch {
	case spec.Direct != nil:
		spec.Direct.ImagesRepo = strings.TrimRight(spec.Direct.ImagesRepo, "/")
	case spec.Unmanaged != nil:
		spec.Unmanaged.ImagesRepo = strings.TrimRight(spec.Unmanaged.ImagesRepo, "/")
	}

	if err := spec.Validate(); err != nil {
		return err
	}

	*s = spec
	return nil
}

func (s *Spec) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.Mode,
			validation.In(registry_const.ModeDirect, registry_const.ModeUnmanaged),
		),
		validation.Field(&s.Direct,
			validation.When(s.Mode == registry_const.ModeDirect,
				validation.NotNil, validation.Required).Else(validation.Nil),
		),
		validation.Field(&s.Unmanaged,
			validation.When(s.Mode == registry_const.ModeUnmanaged,
				validation.NotNil, validation.Required).Else(validation.Nil),
		),
	)
}

func (s *UnmanagedModeSpec) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.ImagesRepo, validation.Required),
		validation.Field(&s.Scheme, validation.In(SchemeHTTP, SchemeHTTPS)),
		validation.Field(&s.Username, validation.When(s.Password != "", validation.Required)),
		validation.Field(&s.Password, validation.When(s.Username != "", validation.Required)),
		validation.Field(&s.CA, validation.When(s.Scheme == SchemeHTTP, validation.Empty)),
	)
}

func (s *DirectModeSpec) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.ImagesRepo, validation.Required),
		validation.Field(&s.Scheme, validation.In(SchemeHTTP, SchemeHTTPS)),
		validation.Field(&s.Username, validation.When(s.Password != "", validation.Required)),
		validation.Field(&s.Password, validation.When(s.Username != "", validation.Required)),
		validation.Field(&s.CA, validation.When(s.Scheme == SchemeHTTP, validation.Empty)),
	)
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

func SchemeFromString(scheme string) Scheme {
	if strings.EqualFold(scheme, SchemeHTTP) {
		return SchemeHTTP
	}
	return SchemeHTTPS
}
