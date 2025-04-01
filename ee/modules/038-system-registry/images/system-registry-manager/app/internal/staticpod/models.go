/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"net/http"

	nodeservices "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/node-services"
	validation "github.com/go-ozzo/ozzo-validation"
)

type NodeServicesConfigModel struct {
	Version string              `json:"version"`
	Config  nodeservices.Config `json:"config"`
}

func (config *NodeServicesConfigModel) Validate() error {
	return validation.ValidateStruct(config,
		validation.Field(&config.Config, validation.Required),
	)
}

func (cfg NodeServicesConfigModel) Bind(r *http.Request) error {
	return cfg.Validate()
}

// changesModel represents a model to track applied changes
type changesModel struct {
	Distribution bool `json:",omitempty"` // Indicates changes in the distribution configuration.
	Auth         bool `json:",omitempty"` // Indicates changes in the authentication system.
	PKI          bool `json:",omitempty"` // Indicates changes in the public key infrastructure.
	Pod          bool `json:",omitempty"` // Indicates changes in the pod setup.
	Mirrorer     bool `json:",omitempty"` // Indicates changes in the mirrorer configuration.
}
