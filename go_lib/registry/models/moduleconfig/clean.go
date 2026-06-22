/*
Copyright 2026 Flant JSC

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

	validation "github.com/go-ozzo/ozzo-validation/v4"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// CacheSettings mirrors moduleConfig/registry .spec.settings.cache.
type CacheSettings struct {
	Enabled      bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	TTL          string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
	StorageSize  string `json:"storageSize,omitempty" yaml:"storageSize,omitempty"`
	StorageClass string `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	Publish      bool   `json:"publish,omitempty" yaml:"publish,omitempty"`
}

// UpstreamCredentials mirrors .spec.settings.upstream.credentials.
type UpstreamCredentials struct {
	Username  string `json:"username,omitempty" yaml:"username,omitempty"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty"`
	DockerCfg string `json:"dockerCfg,omitempty" yaml:"dockerCfg,omitempty"`
}

// UpstreamSettings mirrors .spec.settings.upstream. Absent (nil) means air-gap.
type UpstreamSettings struct {
	Host        string               `json:"host" yaml:"host"`
	Path        string               `json:"path,omitempty" yaml:"path,omitempty"`
	Scheme      constant.SchemeType  `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	CA          string               `json:"ca,omitempty" yaml:"ca,omitempty"`
	Credentials *UpstreamCredentials `json:"credentials,omitempty" yaml:"credentials,omitempty"`
}

// CleanSettings is .spec.settings of moduleConfig/registry (the clean model).
type CleanSettings struct {
	Cache    CacheSettings     `json:"cache" yaml:"cache"`
	Upstream *UpstreamSettings `json:"upstream,omitempty" yaml:"upstream,omitempty"`
}

// RegistryModuleConfig is the parsed moduleConfig name=registry: module enable
// flag + clean settings.
type RegistryModuleConfig struct {
	Enabled  *bool
	Settings CleanSettings
}

// IsUnmanaged reports whether the registry module is explicitly disabled (BYO
// external registry via initConfiguration.deckhouse.imagesRepo).
func (m RegistryModuleConfig) IsUnmanaged() bool {
	return m.Enabled != nil && !*m.Enabled
}

var _ validation.Validatable = CleanSettings{}

// Validate enforces the cache×upstream matrix.
func (s CleanSettings) Validate() error {
	if !s.Cache.Enabled && s.Upstream == nil {
		return fmt.Errorf("registry settings: either upstream must be set or cache.enabled must be true (no cache and no upstream leaves nowhere to pull images from)")
	}
	if s.Cache.Enabled && s.Cache.StorageSize == "" {
		return fmt.Errorf("registry settings: cache.storageSize is required when cache.enabled is true")
	}
	return validation.ValidateStruct(&s,
		validation.Field(&s.Upstream),
	)
}

var _ validation.Validatable = UpstreamSettings{}

// Validate enforces upstream.host required and scheme enum.
func (u UpstreamSettings) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Host, validation.Required.Error("upstream.host is required when upstream is set")),
		validation.Field(&u.Scheme, validation.In(constant.SchemeHTTP, constant.SchemeHTTPS, constant.SchemeType(""))),
	)
}
