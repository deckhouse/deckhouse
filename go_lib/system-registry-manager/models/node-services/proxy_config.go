/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type ProxyConfig struct {
	HTTP    string `json:"http,omitempty" yaml:"http,omitempty"`
	HTTPS   string `json:"https,omitempty" yaml:"https,omitempty"`
	NoProxy string `json:"no_proxy,omitempty" yaml:"no_proxy,omitempty"`
}

func (p ProxyConfig) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.HTTP, validation.Required),
		validation.Field(&p.HTTPS, validation.Required),
		validation.Field(&p.NoProxy, validation.Required),
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
