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

package types

import (
	"encoding/json"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type DeckhouseSettings struct {
	Mode      registry_const.ModeType `json:"mode" yaml:"mode"`
	Direct    *DirectModeSettings     `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *UnmanagedModeSettings  `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
}

type DirectModeSettings struct {
	RegistrySettings
}

type UnmanagedModeSettings struct {
	RegistrySettings
}

type RegistrySettings struct {
	ImagesRepo string                       `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     SchemeType                   `json:"scheme" yaml:"scheme"`
	CA         string                       `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string                       `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string                       `json:"password,omitempty" yaml:"password,omitempty"`
	License    string                       `json:"license,omitempty" yaml:"license,omitempty"`
	CheckMode  registry_const.CheckModeType `json:"checkMode,omitempty" yaml:"checkMode,omitempty"`
}

func (settings *DeckhouseSettings) Correct() {
	switch {
	case settings.Direct != nil:
		settings.Direct.ImagesRepo = strings.TrimRight(settings.Direct.ImagesRepo, "/")
	case settings.Unmanaged != nil:
		settings.Unmanaged.ImagesRepo = strings.TrimRight(settings.Unmanaged.ImagesRepo, "/")
	}
}

func (settings DeckhouseSettings) ToMap() (map[string]interface{}, error) {
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal deckhouse registry settings: %w", err)
	}

	var ret map[string]interface{}
	if err := json.Unmarshal(data, &ret); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deckhouse registry settings: %w", err)
	}
	return ret, nil
}

func (settings DeckhouseSettings) Validate() error {
	return validation.ValidateStruct(&settings,
		validation.Field(&settings.Mode,
			validation.In(registry_const.ModeDirect, registry_const.ModeUnmanaged).
				Error(fmt.Sprintf("Unknown registry mode: %s", settings.Mode)),
		),
		validation.Field(&settings.Direct,
			validation.When(settings.Mode == registry_const.ModeDirect,
				validation.NotNil,
				validation.Required.Error("Field 'direct' is required when mode is 'Direct'"),
			).Else(
				validation.Nil.Error("Field 'direct' must be empty when mode is not 'Direct'"),
			),
		),
		validation.Field(&settings.Unmanaged,
			validation.When(settings.Mode == registry_const.ModeUnmanaged,
				validation.NotNil,
				validation.Required.Error("Field 'unmanaged' is required when mode is 'Unmanaged'"),
			).Else(
				validation.Nil.Error("Field 'unmanaged' must be empty when mode is not 'Unmanaged'"),
			),
		),
	)
}

func (settings RegistrySettings) Validate() error {
	return validation.ValidateStruct(&settings,
		validation.Field(&settings.CheckMode,
			validation.When(settings.CheckMode != "",
				validation.In(registry_const.CheckModeDefault, registry_const.CheckModeRelax).
					Error(fmt.Sprintf("unknown registry check mode: %s", settings.CheckMode))),
		),
		validation.Field(&settings.ImagesRepo,
			validation.Required.Error("Field 'imagesRepo' is required"),
		),
		validation.Field(&settings.Scheme,
			validation.In(SchemeHTTP, SchemeHTTPS).
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", settings.Scheme)),
		),
		validation.Field(&settings.Username,
			validation.When(settings.Password != "",
				validation.Required.Error("Username is required when password is provided"),
			),
		),
		validation.Field(&settings.Password,
			validation.When(settings.Username != "",
				validation.Required.Error("Password is required when username is provided"),
			),
		),
		validation.Field(&settings.License,
			validation.When(settings.Username != "" || settings.Password != "",
				validation.Empty.Error("License field must be empty when using credentials (username/password)"),
			),
		),
		validation.Field(&settings.CA,
			validation.When(settings.Scheme == SchemeHTTP,
				validation.Empty.Error("CA is not allowed when scheme is 'HTTP'"),
			),
		),
	)
}
