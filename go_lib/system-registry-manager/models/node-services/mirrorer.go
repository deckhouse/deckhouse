/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package nodeservices

import (
	validation "github.com/go-ozzo/ozzo-validation"
)

type Mirrorer struct {
	UserPuller User     `json:"user_puller,omitempty" yaml:"user_puller,omitempty"`
	UserPusher User     `json:"user_pusher,omitempty" yaml:"user_pusher,omitempty"`
	Upstreams  []string `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`
}

func (m Mirrorer) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.UserPuller, validation.Required),
		validation.Field(&m.UserPusher, validation.Required),
	)
}
