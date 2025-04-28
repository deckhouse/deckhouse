/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	inclusterproxy "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-service"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type Params struct {
	Generation int64
	Mode       registry_const.ModeType
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
	Scheme     string
	CA         string
}

type Inputs struct {
	Params Params

	IngressCA string

	PKI             pki.Inputs
	Secrets         secrets.Inputs
	Users           users.Inputs
	NodeServices    nodeservices.Inputs
	InClusterProxy  inclusterproxy.Inputs
	RegistryService registryservice.Inputs
}

type Values struct {
	Hash  string `json:"hash,omitempty"`
	State State  `json:"state,omitempty"`
}
