/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

// Config represents the configuration
type Config struct {
	Registry Registry `json:"registry,omitempty" yaml:"registry,omitempty"`
	PKI      PKI      `json:"pki,omitempty" yaml:"pki,omitempty"`
	Proxy    *Proxy   `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

func (config *Config) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Registry, validation.Required),
		validation.Field(&config.PKI, validation.Required),
		validation.Field(&config.Proxy),
	)
}
