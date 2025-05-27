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

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type ProxyMode struct {
	Upstream UpstreamRegistry `json:"upstream" yaml:"upstream"`

	UpstreamRegistryCACert string `json:"upstream_registry_ca,omitempty" yaml:"upstream_registry_ca,omitempty"`
}

func (proxyMode ProxyMode) Validate() error {
	return validation.ValidateStruct(&proxyMode,
		validation.Field(&proxyMode.Upstream, validation.Required),
	)
}

// UpstreamRegistry holds upstream registry configuration details
type UpstreamRegistry struct {
	Scheme   string  `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Host     string  `json:"host,omitempty" yaml:"host,omitempty"`
	Path     string  `json:"path,omitempty" yaml:"path,omitempty"`
	User     string  `json:"user,omitempty" yaml:"user,omitempty"`
	Password string  `json:"password,omitempty" yaml:"password,omitempty"`
	TTL      *string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

func (upstream UpstreamRegistry) Validate() error {
	return validation.ValidateStruct(&upstream,
		validation.Field(&upstream.Scheme, validation.Required),
		validation.Field(&upstream.Host, validation.Required),
		validation.Field(&upstream.Path, validation.Required),
		validation.Field(&upstream.User, validation.Required),
		validation.Field(&upstream.Password, validation.Required),
	)
}
