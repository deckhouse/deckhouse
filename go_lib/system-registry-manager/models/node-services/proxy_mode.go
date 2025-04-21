/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type ProxyMode struct {
	Upstream UpstreamRegistry `json:"upstream" yaml:"upstream"`

	UpstreamRegistryCACert string `json:"upstream_registry_ca,omitempty" yaml:"upstream_registry_ca,omitempty"`
}

func (proxyMode *ProxyMode) Validate() error {
	return validation.ValidateStruct(proxyMode,
		validation.Field(&proxyMode.Upstream, validation.Required),
	)
}
