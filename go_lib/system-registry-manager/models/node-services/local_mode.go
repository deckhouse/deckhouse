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

type LocalMode struct {
	UserRW     User     `json:"user_rw" yaml:"user_rw"`
	UserPuller User     `json:"user_puller" yaml:"user_puller"`
	UserPusher User     `json:"user_pusher" yaml:"user_pusher"`
	Upstreams  []string `json:"upstreams,omitempty" yaml:"upstreams,omitempty"`

	IngressClientCACert string `json:"ingress_client_ca,omitempty" yaml:"ingress_client_ca,omitempty"`
}

func (localModel LocalMode) Validate() error {
	return validation.ValidateStruct(&localModel,
		validation.Field(&localModel.UserRW, validation.Required),
		validation.Field(&localModel.UserPuller, validation.Required),
		validation.Field(&localModel.UserPusher, validation.Required),
	)
}
