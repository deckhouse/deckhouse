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

package orchestrator

import (
	"crypto/x509"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouse-registry"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bashible"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bootstrap"
	inclusterproxy "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/incluster-proxy"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/pki"
	registryservice "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-service"
	registryswither "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/registry-switcher"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/secrets"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/users"
)

type Params struct {
	Generation int64
	Mode       registry_const.ModeType
	ImagesRepo string
	UserName   string
	Password   string
	TTL        string
	Scheme     string
	CA         *x509.Certificate // optional

	CheckMode registry_const.CheckModeType
}

type Inputs struct {
	Params         Params
	RegistrySecret deckhouse_registry.Config
	Bootstrap      bootstrap.Inputs

	PKI              pki.Inputs
	Secrets          secrets.Inputs
	Users            users.Inputs
	InClusterProxy   inclusterproxy.Inputs
	RegistryService  registryservice.Inputs
	Bashible         bashible.Inputs
	RegistrySwitcher registryswither.Inputs
	CheckerStatus    checker.Status
}

type Values struct {
	Hash  string `json:"hash,omitempty"`
	State State  `json:"state,omitempty"`
}

func (p Params) Validate() error {
	if p.Mode == registry_const.ModeUnmanaged && p.ImagesRepo == "" {
		// Skip validation for Unmanaged mode if it's not configurable
		return nil
	}

	switch p.Mode {
	case registry_const.ModeDirect, registry_const.ModeUnmanaged:
		return validation.ValidateStruct(&p,
			validation.Field(&p.ImagesRepo, validation.Required),
			validation.Field(&p.Scheme, validation.In("HTTP", "HTTPS")),
			validation.Field(&p.UserName, validation.When(p.Password != "", validation.Required)),
			validation.Field(&p.Password, validation.When(p.UserName != "", validation.Required)),
		)
	}
	return fmt.Errorf("Unknown registry mode: %q", p.Mode)
}
