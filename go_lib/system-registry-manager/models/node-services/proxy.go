/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type Proxy struct {
	HTTP    string `json:"http,omitempty" yaml:"http,omitempty"`
	HTTPS   string `json:"https,omitempty" yaml:"https,omitempty"`
	NoProxy string `json:"no_proxy,omitempty" yaml:"no_proxy,omitempty"`
}

func (p Proxy) Validate() error {
	return validation.ValidateStruct(&p,
		validation.Field(&p.HTTP, validation.Required),
		validation.Field(&p.HTTPS, validation.Required),
		validation.Field(&p.NoProxy, validation.Required),
	)
}
