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

func (proxyConfig ProxyConfig) Validate() error {
	return validation.ValidateStruct(&proxyConfig,
		validation.Field(&proxyConfig.HTTP, validation.Required),
		validation.Field(&proxyConfig.HTTPS, validation.Required),
		validation.Field(&proxyConfig.NoProxy, validation.Required),
	)
}
