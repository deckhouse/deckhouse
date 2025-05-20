/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package orchestrator

import (
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/node-services"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/registry-service"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/ee/modules/038-system-registry/hooks/orchestrator/users"
	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/models/deckhouse-registry"
)

type Params struct {
	Generation int64                   `json:"generation,omitempty"`
	Mode       registry_const.ModeType `json:"mode,omitempty"`
	ImagesRepo string                  `json:"images_repo,omitempty"`
	UserName   string                  `json:"username,omitempty"`
	Password   string                  `json:"password,omitempty"`
	TTL        string                  `json:"ttl,omitempty"`
	Scheme     string                  `json:"scheme,omitempty"`
	CA         string                  `json:"ca,omitempty"`
}

type Inputs struct {
	Params          Params
	RegistrySecret  deckhouse_registry.Secret
	IngressClientCA string

	PKI             pki.Inputs
	Secrets         secrets.Inputs
	Users           users.Inputs
	NodeServices    nodeservices.Inputs
	InClusterProxy  inclusterproxy.Inputs
	RegistryService registryservice.Inputs
	Bashible        bashible.Inputs
}

type Values struct {
	Hash  string `json:"hash,omitempty"`
	State State  `json:"state,omitempty"`
}
