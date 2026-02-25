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
	"regexp"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

var (
	_ validation.Validatable = DeckhouseSettings{}
	_ validation.Validatable = RegistrySettings{}
	_ validation.Validatable = ProxySettings{}
)

var (
	imagesRepoRegexp      = regexp.MustCompile(`^[0-9a-zA-Z\.\-]+(\:[0-9]{1,5})?(\/[0-9a-zA-Z\.\-\_]+)*$`)
	errorImagesRepoRegexp = fmt.Errorf("does not match the regexp pattern: `%s`", imagesRepoRegexp.String())
)

type DeckhouseSettings struct {
	Mode      constant.ModeType `json:"mode" yaml:"mode"`
	Direct    *RegistrySettings `json:"direct,omitempty" yaml:"direct,omitempty"`
	Unmanaged *RegistrySettings `json:"unmanaged,omitempty" yaml:"unmanaged,omitempty"`
	Proxy     *ProxySettings    `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

func New(mode constant.ModeType) DeckhouseSettings {
	if mode == "" {
		mode = constant.ModeDirect
	}

	settings := DeckhouseSettings{
		Mode: mode,
	}

	registrySettings := NewRegistrySettings()

	switch settings.Mode {
	case constant.ModeDirect:
		settings.Direct = &registrySettings

	case constant.ModeUnmanaged:
		settings.Unmanaged = &registrySettings

	case constant.ModeProxy:
		settings.Proxy = &ProxySettings{
			RegistrySettings: registrySettings,
		}
	}

	return settings
}

func (s DeckhouseSettings) ToMap() map[string]any {
	result := map[string]any{
		"mode": string(s.Mode),
	}

	if s.Direct != nil {
		result["direct"] = s.Direct.ToMap()
	}

	if s.Unmanaged != nil {
		result["unmanaged"] = s.Unmanaged.ToMap()
	}

	if s.Proxy != nil {
		result["proxy"] = s.Proxy.ToMap()
	}

	return result
}

func (s DeckhouseSettings) Merge(other *DeckhouseSettings) DeckhouseSettings {
	out := *s.DeepCopy()

	if other == nil {
		return out
	}

	out.Mode = other.Mode

	if other.Direct != nil {
		if out.Direct == nil {
			out.Direct = other.Direct.DeepCopy()
		} else {
			merged := out.Direct.Merge(other.Direct)
			out.Direct = &merged
		}
	}

	if other.Unmanaged != nil {
		if out.Unmanaged == nil {
			out.Unmanaged = other.Unmanaged.DeepCopy()
		} else {
			merged := out.Unmanaged.Merge(other.Unmanaged)
			out.Unmanaged = &merged
		}
	}

	if other.Proxy != nil {
		if out.Proxy == nil {
			out.Proxy = other.Proxy.DeepCopy()
		} else {
			merged := out.Proxy.Merge(other.Proxy)
			out.Proxy = &merged
		}
	}

	return out
}

func (s DeckhouseSettings) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.Mode,
			validation.Required.
				Error(fmt.Sprintf("Unknown registry mode: %s", s.Mode)),
			validation.In(constant.ModeDirect, constant.ModeUnmanaged, constant.ModeProxy, constant.ModeLocal).
				Error(fmt.Sprintf("Unknown registry mode: %s", s.Mode)),
		),
		validation.Field(&s.Direct,
			validation.When(s.Mode == constant.ModeDirect,
				validation.NotNil,
				validation.Required.Error("Section 'direct' is required when mode is 'Direct'"),
			).Else(
				validation.Nil.Error("Section 'direct' must be empty when mode is not 'Direct'"),
			),
		),
		validation.Field(&s.Unmanaged,
			validation.When(s.Mode == constant.ModeUnmanaged,
				validation.NotNil,
				validation.Required.Error("Section 'unmanaged' is required when mode is 'Unmanaged'"),
			).Else(
				validation.Nil.Error("Section 'unmanaged' must be empty when mode is not 'Unmanaged'"),
			),
		),
		validation.Field(&s.Proxy,
			validation.When(s.Mode == constant.ModeProxy,
				validation.NotNil,
				validation.Required.Error("Section 'proxy' is required when mode is 'Proxy'"),
			).Else(
				validation.Nil.Error("Section 'proxy' must be empty when mode is not 'Proxy'"),
			),
		),
	)
}

func (s *DeckhouseSettings) DeepCopyInto(out *DeckhouseSettings) {
	*out = *s

	if s.Direct != nil {
		out.Direct = new(RegistrySettings)
		s.Direct.DeepCopyInto(out.Direct)
	}

	if s.Unmanaged != nil {
		out.Unmanaged = new(RegistrySettings)
		s.Unmanaged.DeepCopyInto(out.Unmanaged)
	}

	if s.Proxy != nil {
		out.Proxy = new(ProxySettings)
		s.Proxy.DeepCopyInto(out.Proxy)
	}
}

func (s *DeckhouseSettings) DeepCopy() *DeckhouseSettings {
	if s == nil {
		return nil
	}
	out := new(DeckhouseSettings)
	s.DeepCopyInto(out)
	return out
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

func NewRegistrySettings() RegistrySettings {
	return RegistrySettings{
		ImagesRepo: constant.DefaultImagesRepo,
		Scheme:     constant.DefaultScheme,
	}
}

func (s RegistrySettings) ToMap() map[string]any {
	result := map[string]any{
		"imagesRepo": s.ImagesRepo,
		"scheme":     string(s.Scheme),
	}

	if s.CA != "" {
		result["ca"] = s.CA
	}

	if s.Username != "" {
		result["username"] = s.Username
	}

	if s.Password != "" {
		result["password"] = s.Password
	}

	if s.License != "" {
		result["license"] = s.License
	}

	if s.CheckMode != "" {
		result["checkMode"] = string(s.CheckMode)
	}

	return result
}

func (s RegistrySettings) Merge(other *RegistrySettings) RegistrySettings {
	out := *s.DeepCopy()

	if other == nil {
		return out
	}

	if other.ImagesRepo != "" {
		out.ImagesRepo = other.ImagesRepo
	}

	if other.Scheme != "" {
		out.Scheme = other.Scheme
	}

	if other.CA != "" {
		out.CA = other.CA
	}

	if other.Username != "" {
		out.Username = other.Username
	}

	if other.Password != "" {
		out.Password = other.Password
	}

	if other.License != "" {
		out.License = other.License
	}

	if other.CheckMode != "" {
		out.CheckMode = other.CheckMode
	}

	return out
}

func (s RegistrySettings) Validate() error {
	return validation.ValidateStruct(&s,
		validation.Field(&s.CheckMode,
			validation.In(constant.CheckModeDefault, constant.CheckModeRelax).
				Error(fmt.Sprintf("unknown registry check mode: %s", s.CheckMode)),
		),
		validation.Field(&s.ImagesRepo,
			validation.Required.Error("Field 'imagesRepo' is required"),
			validation.Match(imagesRepoRegexp).Error(errorImagesRepoRegexp.Error()),
		),
		validation.Field(&s.Scheme,
			validation.Required.
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", s.Scheme)),
			validation.In(constant.SchemeHTTP, constant.SchemeHTTPS).
				Error(fmt.Sprintf("Invalid scheme '%s'; expected 'HTTP' or 'HTTPS'", s.Scheme)),
		),
		validation.Field(&s.Username,
			validation.When(s.Password != "",
				validation.Required.Error("Username is required when password is provided"),
			),
		),
		validation.Field(&s.Password,
			validation.When(s.Username != "",
				validation.Required.Error("Password is required when username is provided"),
			),
		),
		validation.Field(&s.License,
			validation.When(s.Username != "" || s.Password != "",
				validation.Empty.Error("License field must be empty when using credentials (username/password)"),
			),
		),
		validation.Field(&s.CA,
			validation.When(s.Scheme == constant.SchemeHTTP,
				validation.Empty.Error("CA is not allowed when scheme is 'HTTP'"),
			),
		),
	)
}

func (s *RegistrySettings) DeepCopyInto(out *RegistrySettings) {
	*out = *s
}

func (s *RegistrySettings) DeepCopy() *RegistrySettings {
	if s == nil {
		return nil
	}
	out := new(RegistrySettings)
	s.DeepCopyInto(out)
	return out
}

type ProxySettings struct {
	RegistrySettings
	TTL string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

func (s ProxySettings) ToMap() map[string]any {
	ret := s.RegistrySettings.ToMap()

	if s.TTL != "" {
		ret["ttl"] = s.TTL
	}

	return ret
}

func (s ProxySettings) Merge(other *ProxySettings) ProxySettings {
	out := *s.DeepCopy()

	if other == nil {
		return out
	}

	out.RegistrySettings = out.RegistrySettings.
		Merge(&other.RegistrySettings)

	if other.TTL != "" {
		out.TTL = other.TTL
	}

	return out
}

func (s ProxySettings) Validate() error {
	if err := s.RegistrySettings.Validate(); err != nil {
		return err
	}

	ttl := s.TTL
	if len(ttl) > 0 {
		if err := validateTTL(ttl); err != nil {
			return fmt.Errorf("invalid ttl format %q: %w", ttl, err)
		}
	}
	return nil
}

func (s *ProxySettings) DeepCopyInto(out *ProxySettings) {
	*out = *s
	s.RegistrySettings.DeepCopyInto(&out.RegistrySettings)
}

func (s *ProxySettings) DeepCopy() *ProxySettings {
	if s == nil {
		return nil
	}
	out := new(ProxySettings)
	s.DeepCopyInto(out)
	return out
}
