/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type LocalMode struct {
	UserRW     User     `json:"user_rw" yaml:"user_rw"`
	UserPuller User     `json:"user_puller" yaml:"user_puller"`
	UserPusher User     `json:"user_pusher" yaml:"user_pusher"`
	Upstreams  []string `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`

	IngressClientCACert string `json:"ingress_client_ca,omitempty" yaml:"ingress_client_ca,omitempty"`
}

func (localModel *LocalMode) Validate() error {
	return validation.ValidateStruct(localModel,
		validation.Field(&localModel.UserRW, validation.Required),
		validation.Field(&localModel.UserPuller, validation.Required),
		validation.Field(&localModel.UserPusher, validation.Required),
	)
}
