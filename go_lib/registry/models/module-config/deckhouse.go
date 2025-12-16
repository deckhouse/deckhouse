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

package moduleconfig

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

type DeckhouseSettings struct {
	Mode      constant.ModeType `json:"mode" yaml:"mode"`
	Direct    *RegistrySettings `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *RegistrySettings `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
}

func (settings DeckhouseSettings) ToMap() map[string]any {
	result := map[string]any{
		"mode": string(settings.Mode),
	}

	if settings.Direct != nil {
		result["direct"] = settings.Direct.ToMap()
	}

	if settings.Unmanaged != nil {
		result["unmanaged"] = settings.Unmanaged.ToMap()
	}

	return result
}

func (settings *DeckhouseSettings) ApplySettings(userSettings DeckhouseSettings) {
	*settings = DeckhouseSettings{
		Mode: userSettings.Mode,
	}

	switch settings.Mode {
	case constant.ModeDirect:
		var direct RegistrySettings
		direct.ApplySettings(userSettings.Direct)

		settings.Direct = &direct

	case constant.ModeUnmanaged:
		var unmanaged RegistrySettings
		unmanaged.ApplySettings(userSettings.Unmanaged)

		settings.Unmanaged = &unmanaged
	}
}

func (settings DeckhouseSettings) Validate() error {
	return validation.ValidateStruct(&settings,
		validation.Field(&settings.Mode,
			validation.Required.
				Error(fmt.Sprintf("Unknown registry mode: %s", settings.Mode)),
			validation.In(constant.ModeDirect, constant.ModeUnmanaged).
				Error(fmt.Sprintf("Unknown registry mode: %s", settings.Mode)),
		),
		validation.Field(&settings.Direct,
			validation.When(settings.Mode == constant.ModeDirect,
				validation.NotNil,
				validation.Required.Error("Section 'direct' is required when mode is 'Direct'"),
			).Else(
				validation.Nil.Error("Section 'direct' must be empty when mode is not 'Direct'"),
			),
		),
		validation.Field(&settings.Unmanaged,
			validation.When(settings.Mode == constant.ModeUnmanaged,
				validation.NotNil,
				validation.Required.Error("Section 'unmanaged' is required when mode is 'Unmanaged'"),
			).Else(
				validation.Nil.Error("Section 'unmanaged' must be empty when mode is not 'Unmanaged'"),
			),
		),
	)
}

type RegistrySettings struct {
	ImagesRepo string                 `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     constant.SchemeType    `json:"scheme" yaml:"scheme"`
	CA         string                 `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string                 `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string                 `json:"password,omitempty" yaml:"password,omitempty"`
	License    string                 `json:"license,omitempty" yaml:"license,omitempty"`
	CheckMode  constant.CheckModeType `json:"checkMode,omitempty" yaml:"checkMode,omitempty"`
}

func (settings RegistrySettings) ToMap() map[string]any {
	result := map[string]any{
		"imagesRepo": settings.ImagesRepo,
		"scheme":     string(settings.Scheme),
	}

	if settings.CA != "" {
		result["ca"] = settings.CA
	}

	if settings.Username != "" {
		result["username"] = settings.Username
	}

	if settings.Password != "" {
		result["password"] = settings.Password
	}

	if settings.License != "" {
		result["license"] = settings.License
	}

	if settings.CheckMode != "" {
		result["checkMode"] = string(settings.CheckMode)
	}

	return result
}

func (settings *RegistrySettings) ApplySettings(userSettings *RegistrySettings) {
	// Set default
	*settings = RegistrySettings{
		ImagesRepo: constant.DefaultImagesRepo,
		Scheme:     constant.DefaultScheme,
	}

	if userSettings == nil {
		return
	}

	// Set user settings
	userSettings.ImagesRepo = strings.TrimRight(strings.TrimSpace(userSettings.ImagesRepo), "/")
	if userSettings.ImagesRepo != "" {
		settings.ImagesRepo = userSettings.ImagesRepo
	}

	if userSettings.Scheme != "" {
		settings.Scheme = userSettings.Scheme
	}

	if userSettings.CA != "" {
		settings.CA = userSettings.CA
	}

	if userSettings.Username != "" {
		settings.Username = userSettings.Username
	}

	if userSettings.Password != "" {
		settings.Password = userSettings.Password
	}

	if userSettings.License != "" {
		settings.License = userSettings.License
	}

	if userSettings.CheckMode != "" {
		settings.CheckMode = userSettings.CheckMode
	}
}

func (settings RegistrySettings) Validate() error {
	return validation.ValidateStruct(&settings,
		validation.Field(&settings.CheckMode,
			validation.In(constant.CheckModeDefault, constant.CheckModeRelax).
				Error(fmt.Sprintf("unknown registry check mode: %s", settings.CheckMode)),
		),
		validation.Field(&settings.ImagesRepo,
			validation.Required.Error("Field 'imagesRepo' is required"),
		),
		validation.Field(&settings.Scheme,
			validation.Required.
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", settings.Scheme)),
			validation.In(constant.SchemeHTTP, constant.SchemeHTTPS).
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
			validation.When(settings.Scheme == constant.SchemeHTTP,
				validation.Empty.Error("CA is not allowed when scheme is 'HTTP'"),
			),
		),
	)
}
