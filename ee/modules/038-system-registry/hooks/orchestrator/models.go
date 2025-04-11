/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type Params struct {
	Mode       registry_const.ModeType
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
}

type State struct {
	Mode       registry_const.ModeType
	TargetMode registry_const.ModeType

	PKIVersion   string
	UsersVersion string
}
