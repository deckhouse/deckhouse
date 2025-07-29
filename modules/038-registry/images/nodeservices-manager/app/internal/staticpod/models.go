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

package staticpod

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	nodeservices "github.com/deckhouse/deckhouse/go_lib/registry/models/node-services"
)

type NodeServicesConfigModel struct {
	Version string              `json:"version"`
	Config  nodeservices.Config `json:"config"`
}

func (value NodeServicesConfigModel) Validate() error {
	return validation.ValidateStruct(&value,
		validation.Field(&value.Config, validation.Required),
	)
}

// changesModel represents a model to track applied changes
type changesModel struct {
	Distribution bool `json:",omitempty"` // Indicates changes in the distribution configuration.
	Auth         bool `json:",omitempty"` // Indicates changes in the authentication system.
	PKI          bool `json:",omitempty"` // Indicates changes in the public key infrastructure.
	Pod          bool `json:",omitempty"` // Indicates changes in the pod setup.
	Mirrorer     bool `json:",omitempty"` // Indicates changes in the mirrorer configuration.
}

// HasChanges checks if any field is true.
func (c changesModel) HasChanges() bool {
	return c.Distribution || c.Auth || c.PKI || c.Pod || c.Mirrorer
}
