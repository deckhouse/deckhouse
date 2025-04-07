/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

// UpstreamRegistry holds upstream registry configuration details
type UpstreamRegistry struct {
	Scheme   string  `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	Host     string  `json:"host,omitempty" yaml:"host,omitempty"`
	Path     string  `json:"path,omitempty" yaml:"path,omitempty"`
	User     string  `json:"user,omitempty" yaml:"user,omitempty"`
	Password string  `json:"password,omitempty" yaml:"password,omitempty"`
	TTL      *string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

func (u UpstreamRegistry) Validate() error {
	return validation.ValidateStruct(&u,
		validation.Field(&u.Scheme, validation.Required),
		validation.Field(&u.Host, validation.Required),
		validation.Field(&u.Path, validation.Required),
		validation.Field(&u.User, validation.Required),
		validation.Field(&u.Password, validation.Required),
	)
}
