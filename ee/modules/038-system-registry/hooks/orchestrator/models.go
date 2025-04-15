/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type Params struct {
	Mode       registry_const.ModeType
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
}

type Inputs struct {
	Params  Params
	PKI     pki.State
	Secrets secrets.State
}

type State struct {
	Mode registry_const.ModeType `json:"mode"`

	PKI     *pki.State     `json:"pki,omitempty"`
	Secrets *secrets.State `json:"secrets,omitempty"`

	UsersVersion string `json:"users_version,omitempty"`
}
