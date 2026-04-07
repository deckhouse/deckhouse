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
	deckhouse_registry "github.com/deckhouse/deckhouse/go_lib/registry/models/deckhouseregistry"
	init_secret "github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
	registry_pki "github.com/deckhouse/deckhouse/go_lib/registry/pki"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/checker"
	"github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/bashible"
	inclusterproxy "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/incluster-proxy"
	nodeservices "github.com/deckhouse/deckhouse/modules/038-registry/hooks/orchestrator/node-services"
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

func (p Params) toState() ParamsState {
	var ca []byte
	if p.CA != nil {
		ca = registry_pki.EncodeCertificate(p.CA)
	}
	return ParamsState{
		Generation: p.Generation,
		Mode:       p.Mode,
		ImagesRepo: p.ImagesRepo,
		UserName:   p.UserName,
		Password:   p.Password,
		TTL:        p.TTL,
		Scheme:     p.Scheme,
		CheckMode:  p.CheckMode,
		CA:         string(ca),
	}
}

type ParamsState struct {
	Generation int64                   `json:"generation,omitempty"`
	Mode       registry_const.ModeType `json:"mode,omitempty"`
	ImagesRepo string                  `json:"images_repo,omitempty"`
	UserName   string                  `json:"user_name,omitempty"`
	Password   string                  `json:"password,omitempty"`
	TTL        string                  `json:"ttl,omitempty"`
	Scheme     string                  `json:"scheme,omitempty"`
	CA         string                  `json:"ca,omitempty"`

	CheckMode registry_const.CheckModeType `json:"check_mode,omitempty"`
}

func (p ParamsState) toParams() (Params, error) {
	var ca *x509.Certificate
	if p.CA != "" {
		var err error
		ca, err = registry_pki.DecodeCertificate([]byte(p.CA))
		if err != nil {
			return Params{}, fmt.Errorf("failed to decode CA certificate: %w", err)
		}
	}
	return Params{
		Generation: p.Generation,
		Mode:       p.Mode,
		ImagesRepo: p.ImagesRepo,
		UserName:   p.UserName,
		Password:   p.Password,
		TTL:        p.TTL,
		Scheme:     p.Scheme,
		CheckMode:  p.CheckMode,
		CA:         ca,
	}, nil
}

type Inputs struct {
	Params          Params
	RegistrySecret  deckhouse_registry.Config
	InitSecret      init_secret.Config
	IngressClientCA *x509.Certificate // optional

	PKI              pki.Inputs
	Secrets          secrets.Inputs
	Users            users.Inputs
	NodeServices     nodeservices.Inputs
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

type InitSecretSnap struct {
	IsExist bool
	Applied bool
	Config  []byte
}

func (p Params) Validate() error {
	if p.Mode == registry_const.ModeUnmanaged && p.ImagesRepo == "" {
		// Skip validation for Unmanaged mode if it's not configurable
		return nil
	}

	switch p.Mode {
	case registry_const.ModeDirect, registry_const.ModeProxy, registry_const.ModeUnmanaged:
		return validation.ValidateStruct(&p,
			validation.Field(&p.ImagesRepo, validation.Required),
			validation.Field(&p.Scheme, validation.In("HTTP", "HTTPS")),
			validation.Field(&p.UserName, validation.When(p.Password != "", validation.Required)),
			validation.Field(&p.Password, validation.When(p.UserName != "", validation.Required)),
		)
	case registry_const.ModeLocal:
		return nil
	}
	return fmt.Errorf("Unknown registry mode: %q", p.Mode)
}
